package main

import (
	"encoding/gob"
	"encoding/hex"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	poffice "popcorn/postOffice"
	messager "popcorn/proto"
	"strconv"
	"sync/atomic"
	"time"
	"unsafe"
)

func clientCommunication(){
	for conn := range postman.CliConns{
		handleCliConn(conn)
	}
}

func handleCliConn(conn *net.Conn){
	log.Warningf("NEW CLIENT REGISTERED | ClientAddr: %s",
		(*conn).RemoteAddr())

	maidenComm := true
	dec := gob.NewDecoder(*conn)
	var clientDock *clientConnDock

	receivedMessageIndicator := 0
	for {

		var m messager.MessageBank

		err := dec.Decode(&m)
		if err != nil{
			if err == io.EOF {
				log.Warnf("EOF | cliAddr: %v", (*conn).RemoteAddr())
				break
			}
			log.Warnf("Gob Decode error | err: %v | cliAddr: %v", err, (*conn).RemoteAddr())
			continue
		}

		if m.GetCmuType() != messager.CommunicationType_Type_Client_To_Cluster{
			log.Warnf("Wrong mType %v", m.GetCmuType())
			continue
		}

		if maidenComm {
			ca := &clientConnDock{
				clientConn: conn,
				m:          make(map[int64]*TXLog),
				enc:        gob.NewEncoder(*conn),
				dec:        gob.NewDecoder(*conn),
			}

			clientDock = ca
			muClientConnNavigator.Lock()
			clientConnNavigator[m.GetClientProposal().GetClientID()]= ca
			muClientConnNavigator.Unlock()

			maidenComm = false
		}

		receivedMessageIndicator++
		if receivedMessageIndicator == BatchSize{

		}

		go asynDisseminateLog(m.GetClientProposal(), clientDock)
		//asynDisseminateLog(m.GetClientProposal(), clientDock)
	}
}

func asynDisseminateLog(proposal *messager.ClientProposal, ca *clientConnDock){

	ok := ValidateMACs(proposal.GetClientID(), proposal.GetTx(), proposal.GetMacs())
	if !ok {
		log.Errorln("validation of txMACs failed")
		return
	}

	txHash, sig, err := signMsgDigest(proposal.GetTx())
	if err != nil{
		log.Errorf("sign failed | err: %v", err)
		return
	}

	log.Infof("NEW PROPOSAL | TS: %v | Tx: %v", proposal.GetTimestamp(), proposal.GetTx())

	msgIndex := atomic.AddInt64(&LogIndex, 1)

	endorsedInfo := append([]byte(strconv.FormatInt(msgIndex, 10)), txHash...)
	infoHash := getDigest(endorsedInfo)
	infoSig, err := PenSign(infoHash[:])
	if err != nil{
		log.Errorln("Sign failed;", err)
		return
	}
	sigShares := [][]byte{infoSig}

	txLog := &TXLog{
		Index:		  msgIndex,
		TimeStamp:    proposal.GetTimestamp(),
		TxHash:       txHash,
		LogIndicator: 1, // leader logs it first
		SigShares:	  sigShares,
		Committed:	  false,
	}

	ca.Lock()
	ca.m[proposal.GetTimestamp()] = txLog
	ca.Unlock()

	logMsg := oneIssueLogInstance(msgIndex, proposal.GetClientID(), proposal.GetTimestamp(), proposal.GetTx(), txHash, sig)

	logCmd := prepareIssueLogMessage(logMsg)

	broadcast(logCmd)

}

func serverCommunication(){

	postPhaseListenerAddress := ServerList[0].Ip + ":" + ServerList[0].Ports[poffice.PortInternalListenerPostPhase]
	postPhaseListener, err := net.Listen("tcp4", postPhaseListenerAddress)

	if err != nil {
		log.Error(err)
		return
	}

	i := 0
	for {
		if ThisServerID == i { i ++ } // skip recording this server Id into the array

		if conn, err := postPhaseListener.Accept(); err == nil {
			go handleServerConn(&conn, i)
		} else {
			log.Error(err)
		}

		i ++
	}
}

func handleServerConn(sConn *net.Conn, sid int) {

	muServerConnNavigator.Lock()
	encoder := gob.NewEncoder(*sConn)
	decoder := gob.NewDecoder(*sConn)

	serConnDock := &serverConnDock{
		serverConn:    sConn,
		serverId:      sid,
		enc:           encoder,
		dec:           decoder,
	}

	serverConnNavigator[sid] = serConnDock

	log.Warningf("New Server Registered: %v -> %v\n", sid, (*sConn).RemoteAddr())
	muServerConnNavigator.Unlock()

	receiveCounter := int64(0)

	for {
		var m messager.MessageBank

		err := serConnDock.dec.Decode(&m)

		counter := atomic.AddInt64(&receiveCounter, 1)

		log.Debugf("server %v | len: %v | received: %+v", sid, unsafe.Sizeof(m), m)

		if err == io.EOF{
			log.Errorf("%v | server %v closed connection | err: %v", time.Now(), sid, err)
			break
		}

		if err != nil{
			log.Errorf("Gob Decode Err: %v | conn with ser: %v | remoteAddr: %v | Now # %v", err, sid, (*sConn).RemoteAddr(), counter)
			continue
		}

		if sr := m.GetServerReplyCmd(); sr != nil {
			go handlingServerReply(sr)
		} else {
			log.Errorf("received message is nil")
		}

	}
}

func handlingServerReply(m *messager.ServerReplyCmd) {


		clientCache := clientConnNavigator[int64(m.GetClientID())]

		if clientCache == nil {
			log.Errorf("corresponding client has not been registered | cid: %v\n", m.GetClientID())
			return
		}

		asynchronousProcessLogReply(m, clientCache)
}

func asynchronousProcessLogReply(m *messager.ServerReplyCmd, ca *clientConnDock){

	if m == nil{
		return
	}

	log.Infof("LOG REPLY | CId: %v | Timestamp: %v | MsgIndex: %v | InfoHash: %v | Sig: %v\n",
		m.GetClientID(), m.GetTimestamp(), m.GetMsgIndex(),
		hex.EncodeToString(m.GetInfoHash()), hex.EncodeToString(m.GetSig()))

	ca.Lock()
	txLog, ok := ca.m[int64(m.GetTimestamp())]
	if !ok {
		log.Debugf("no info in cache; consensus may already reached")
		return
	}

	logIndicator := txLog.LogIndicator

	if logIndicator >= Quorum {
		log.Debugf("THIS SHOULD NEVER HAPPEN| logIndicator: %v | Quorum: %v", logIndicator, Quorum)
		delete(ca.m, int64(m.GetTimestamp()))
		ca.Unlock()
		//continue
		return
	}

	logIndicator++
	ca.m[int64(m.GetTimestamp())].LogIndicator = logIndicator
	sigshares := append(ca.m[int64(m.GetTimestamp())].SigShares, m.GetSig())
	ca.m[int64(m.GetTimestamp())].SigShares = sigshares

	ca.Unlock()

	if logIndicator < Quorum {
		log.Debugf("insufficient votes | TS: %v | logIndicator: %v", m.GetTimestamp(), logIndicator)
		//continue
		return
	}

	//now incremented logIndicator == quorum

	sigThreshed, err := PenRecovery(sigshares, m.GetInfoHash())
	if err != nil{
		log.Errorf("PenRecovery failed | len(sigShares): %v | error: %v", len(sigshares), err)
		return
	}


	txhash := txLog.TxHash

	leaderIssueOrder := prepareLeaderIssueOrderMessage(txhash, m.GetInfoHash(), sigThreshed,
		int64(m.GetMsgIndex()), int64(m.GetClientID()), int64(m.GetTimestamp()))

	//broadcastToSConns(leaderIssueOrder)
	//broadcastToServersMQ <- leaderIssueOrder

	broadcast(leaderIssueOrder)

	if ThreePhaseCommit{
		// FUTURE WORK
		// Three Phase Commit involves prepare, order, commit
		// Three phase Commit is State Machine Safe
	}

	cmtIdx := atomic.AddInt64(&CommitIndex, 1)

	log.Infof("NEW COMMIT | LogIndex: %v | CommitIndex: %v | ClientID: %v | Timestamp: %v\n",
		m.GetMsgIndex(), cmtIdx, m.GetClientID(), m.GetTimestamp())

	ca.Lock()
	delete(ca.m, int64(m.GetTimestamp()))
	log.Debugf("Map size: %v | MsgIndex: %v | TS: %v",
		len(ca.m), m.GetMsgIndex(), m.GetTimestamp())
	ca.Unlock()

	clientConfirmation := prepareClientConfirmation(int64(m.GetTimestamp()), int64(m.GetMsgIndex()), txLog.TxHash)

	err = ca.enc.Encode(clientConfirmation)

	if err != nil{
		log.Errorln(err)
		return
	}
}