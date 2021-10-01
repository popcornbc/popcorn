package main

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	proto "popcorn/proto"
	"popcorn/rsm"
	"sync/atomic"
	"time"
)

func handleCliConn(conn *net.Conn){

	log.Warningf("NEW CLIENT REGISTERED | ClientAddr: %s", (*conn).RemoteAddr())

	maidenComm := true
	dec := gob.NewDecoder(*conn)
	var clientDock *clientConnDock

	//var tools BatchingTools

	for {

		var m proto.ClientProposeEntry

		err := dec.Decode(&m)
		if err != nil{
			if err == io.EOF {
				log.Warnf("EOF | cliAddr: %v", (*conn).RemoteAddr())
				break
			}
			log.Warnf("Gob Decode error | err: %v | cliAddr: %v", err, (*conn).RemoteAddr())
			continue
		}

		if maidenComm {
			ca := &clientConnDock{
				c	: conn,
				m	: make(map[int64]*TXLog),
				enc : gob.NewEncoder(*conn),
				dec : gob.NewDecoder(*conn),
			}

			clientDock = ca
			muClientConnNavigator.Lock()
			clientConnNavigator[m.GetClientID()]= ca
			muClientConnNavigator.Unlock()

			maidenComm = false
		}

		if state.GetCurrentState() != rsm.LEADER{
			//
			//
			//
			// If system state is not leader, no nothing;
			// Future work: pipeline-based approach:
			// Pipes transactions with changeable index.
			return
		}

		if BatchSize == 1 {
			go asynDisseminateLog(&m, clientDock)
		} else {
			go asynPackLogToBatch(&m, clientDock)
		}
	}
}

func asynPackLogToBatch(m *proto.ClientProposeEntry, clientDock *clientConnDock) {
	log.Infof("NEW PROPOSAL | TS: %v | Tx: %v", m.GetTimestamp(), m.GetTransaction())

	ok := ValidateMACs(m.GetClientID(), m.GetTransaction(), m.GetMacs())
	if !ok {
		log.Errorln("validation of txMACs failed")
		return
	}

	//
	// Message index is replaced with the position in the messaging channel
	// The circulated packet is represented by batches. Individual messages
	// in a batch no longer carry message index; however, we kept increasing
	// log index of the system.
	//
	// If the order of messages in a batch matters, one can easily add a feature
	// in OneTx and pass the incremented log index to it.
	state.IncrementLogIndex()

	batching.txQ <- &OneTx{
		tx:        m.Transaction,
		timestamp: m.Timestamp,
		hash:      getDigest([]byte(m.Transaction)),
		conn:      clientDock.c,
		macs:		m.Macs,
	}

	//log.Debugf("orderingInfoHash: %v | leaderSig: %v", hex.EncodeToString(orderingInfoHash), hex.EncodeToString(orderingInfoSig))
	//log.Debugf("commitInfoHash: %v | leaderSig: %v", hex.EncodeToString(commitInfoHash), hex.EncodeToString(commitInfoSig))

}

func disseminateBatches(){
	batchCounter := 0
	var concatTxHash [][]byte
	var batchedEntries []*proto.InBatchEntry

	for oneTx := range batching.txQ {

		concatTxHash = append(concatTxHash, oneTx.hash)

		entry := &proto.InBatchEntry{
			Transaction: oneTx.tx,
			Timestamp:   oneTx.timestamp,
			Macs:        oneTx.macs,
		}

		batchedEntries = append(batchedEntries, entry)

		batchCounter++

		if batchCounter % BatchSize != 0 {
			continue
		}

		//
		// Prepare batch
		//
		//

		batchTxHash := bytes.Join(concatTxHash, nil)

		orderingInfoHash, orderingInfoSig, err := prepareInfoSig(batching.n, batchTxHash, OrderPhase)
		if err != nil{
			log.Errorf("prepareInfoSig of ordering failed | err: %v", err)
			return
		}

		commitInfoHash, commitInfoSig, err := prepareInfoSig(batching.n, batchTxHash, CommitPhase)
		if err != nil{
			log.Errorf("prepareInfoSig of commit failed | err: %v", err)
			return
		}

		txLog := &TXLog{
			Index				: batching.n,
			//ClientConn		: oneTx.conn,
			//TimeStamp			: []int64{m.GetTimestamp()},
			TxHash				: batchTxHash,
			OrderingInfoHash	: orderingInfoHash,
			CommitInfoHash		: commitInfoHash,
			Ordered				: 1,
			Committed			: 0,
			OrderedProofCert	: nil,
			CommittedProofCert	: nil,
			OrderingSigShares	: [][]byte{orderingInfoSig},
			CommitSigShares		: [][]byte{commitInfoSig},
			RSMUpdated			: false,
		}

		// The index of the message is the key of the storage (map)
		batchingCache.Lock()
		batchingCache.m[batching.n] = txLog
		batchingCache.Unlock()

		postBatchedEntries := &proto.LeaderPostBatchedPostEntries{
			BatchIndex:       batching.n,
			SignatureOnBatch: orderingInfoSig,
			BatchedEntries:   batchedEntries,
		}

		go broadcast(postBatchedEntries, PostPhase)

		log.Debugf("=> postEntry broadcast for batch %v", batching.n)

		batching.n++
	}
}

func prepareInfoSig(msgIndex int64, txHash []byte, phase uint) ([]byte, []byte, error){

	cert := ThresholdCertificate{
		MessageIndex:      msgIndex,
		HashOfTransaction: txHash,
		PhaseSymbol:       phase,
	}

	proof, err := serialization(cert)
	if err != nil {
		return nil, nil, err
	}

	infoHash := getDigest(proof)
	infoSig, err := PenSign(infoHash)
	if err != nil {
		return nil, nil, err
	}

	return infoHash, infoSig, nil
}

func asynDisseminateLog(m *proto.ClientProposeEntry, ca *clientConnDock){

	ok := ValidateMACs(m.GetClientID(), m.GetTransaction(), m.GetMacs())
	if !ok {
		log.Errorln("validation of txMACs failed")
		return
	}

	txHash, sig, err := signMsgDigest(m.GetTransaction())
	if err != nil{
		log.Errorf("sign failed | err: %v", err)
		return
	}

	log.Infof("NEW PROPOSAL | TS: %v | Tx: %v", m.GetTimestamp(), m.GetTransaction())

	msgIndex := state.IncrementLogIndex()

/*	cert := ThresholdCertificate{
		MessageIndex:      msgIndex,
		HashOfTransaction: txHash,
		PhaseSymbol:       OrderPhase,
	}

	proof, err := serialization(cert)
	if err != nil {
		log.Errorf("serialize certification failed | err: ", err)
		return
	}

	//endorsedInfo := append([]byte(strconv.FormatInt(msgIndex, 10)), txHash...)
	//infoHash := getDigest(endorsedInfo)

	infoHash := getDigest(proof)*/
	orderingInfoHash, orderingInfoSig, err := prepareInfoSig(msgIndex, txHash, OrderPhase)
	if err != nil{
		log.Errorf("prepareInfoSig of ordering failed | err: %v", err)
		return
	}

	commitInfoHash, commitInfoSig, err := prepareInfoSig(msgIndex, txHash, CommitPhase)
	if err != nil{
		log.Errorf("prepareInfoSig of commit failed | err: %v", err)
		return
	}

	//log.Debugf("orderingInfoHash: %v | leaderSig: %v", hex.EncodeToString(orderingInfoHash), hex.EncodeToString(orderingInfoSig))
	//log.Debugf("commitInfoHash: %v | leaderSig: %v", hex.EncodeToString(commitInfoHash), hex.EncodeToString(commitInfoSig))

	txLog := &TXLog{
		Index				: msgIndex,
		ClientConn			: ca.c,
		TimeStamp			: []int64{m.GetTimestamp()},
		TxHash				: txHash,
		OrderingInfoHash	: orderingInfoHash,
		CommitInfoHash		: commitInfoHash,
		Ordered				: 1,
		Committed			: 0,
		OrderedProofCert	: nil,
		CommittedProofCert	: nil,
		OrderingSigShares	: [][]byte{orderingInfoSig},
		CommitSigShares		: [][]byte{commitInfoSig},
		RSMUpdated			: false,
	}

	// The index of the message is the key of the storage (map)
	ca.Lock()
	ca.m[msgIndex] = txLog
	ca.Unlock()

	postEntry := postPhaseEntry(msgIndex, m.GetClientID(), m.GetTimestamp(), m.GetTransaction(), m.GetMacs(), sig)

	broadcast(postEntry, PostPhase)

	log.Debugf("=> postEntry broadcast for msg %v", msgIndex)
}

func registerServer(sConn *net.Conn, phase int, sid int) {

	serverConnNav.mu.Lock()

	defer serverConnNav.mu.Unlock()

	serverConnNav.n[phase][sid] = &serverConnDock{
		serverId:   sid,
		conn: 		sConn,
		enc:        gob.NewEncoder(*sConn),
		dec:        gob.NewDecoder(*sConn),
	}
}

func handlePostPhaseServerConn(sConn *net.Conn, sid int){
	var postPhaseErrorFlag = errors.New("post phase")

	registerServer(sConn, PostPhase, sid)

	log.Warningf("%v | new server registered | Id: %v -> Addr: %v\n", postPhaseErrorFlag, sid, (*sConn).RemoteAddr())

}

func handleOrderPhaseServerConn(sConn *net.Conn, sid int) {
	// Handle PostReply from servers
	var orderPhaseErrorFlag = errors.New("order phase")

	registerServer(sConn, OrderPhase, sid)

	log.Warningf("%v | new server registered | Id: %v -> Addr: %v\n", orderPhaseErrorFlag, sid, (*sConn).RemoteAddr())

	receiveCounter := int64(0)

	for {
		var m proto.WorkerPostReply

		err := serverConnNav.n[OrderPhase][sid].dec.Decode(&m)

		counter := atomic.AddInt64(&receiveCounter, 1)

		//log.Debugf("server %v | len: %v | received: %+v", sid, unsafe.Sizeof(m), m)

		if err == io.EOF{
			log.Errorf("%v | server %v closed connection | err: %v", time.Now(), sid, err)
			break
		}

		if err != nil{
			log.Errorf("Gob Decode Err: %v | conn with ser: %v | remoteAddr: %v | Now # %v",
				err, sid, (*sConn).RemoteAddr(), counter)
			continue
		}

		if &m != nil {
			go asynHandleServerPostReply(&m)
		} else {
			log.Errorf("received message is nil")
		}
	}
}

func asynHandleServerPostReply(m *proto.WorkerPostReply) {

	if m == nil{
		log.Errorf("received WorkerPostReply is empty")
		return
	}

	clientDock := clientConnNavigator[m.GetClientId()]

	if clientDock == nil {
		log.Errorf("corresponding client has not been registered | cid: %v\n", m.GetClientId())
		return
	}

	log.Infof("WorkerPostReply | CId: %v | MsgIndex: %v | Sig: %v\n",
		m.GetClientId(), m.GetIndex(), hex.EncodeToString(m.GetSignature()))

	clientDock.Lock()
	txLog, ok := clientDock.m[m.GetIndex()]
	if !ok {
		log.Debugf("no info in cache; consensus may already reached")
		return
	}

	orderedIndicator := txLog.Ordered

	if orderedIndicator > Quorum {
		log.Errorf("THIS SHOULD NEVER HAPPEN| logIndicator: %v | Quorum: %v", orderedIndicator, Quorum)
		delete(clientDock.m, m.GetIndex())
		clientDock.Unlock()
		//continue
		return
	}

	orderedIndicator++
	clientDock.m[m.GetIndex()].Ordered = orderedIndicator
	aggregatedSigShares := append(clientDock.m[m.GetIndex()].OrderingSigShares, m.GetSignature())
	clientDock.m[m.GetIndex()].OrderingSigShares = aggregatedSigShares

	clientDock.Unlock()

	switch  {
	case orderedIndicator < Quorum:
		log.Debugf("insufficient votes for ordering | index: %v | orderedIndicator: %v", m.GetIndex(), orderedIndicator)
		//continue
		return
	case orderedIndicator > Quorum:
		log.Debugf("msg %v already broadcasted for ordering", m.GetIndex())
		return
	default:
		sigThreshed, err := PenRecovery(aggregatedSigShares, txLog.OrderingInfoHash)
		if err != nil{
			log.Errorf("PenRecovery failed | len(sigShares): %v | error: %v", len(aggregatedSigShares), err)
			return
		}

		//
		// Uncomment the following part if you need this info in cache.
		//
		// clientDock.Lock()
		// clientDock.m[m.GetIndex()].OrderedProofCert = sigThreshed
		// clientDock.Unlock()

		orderEntry := orderPhaseEntry(m.GetIndex(), m.GetClientId(), sigThreshed)

		broadcast(orderEntry, OrderPhase)

		log.Debugf("=> orderEntry broadcast for msg %v", m.GetIndex())
	}

/*	if orderedIndicator < Quorum {
		log.Debugf("insufficient votes for ordering | index: %v | orderedIndicator: %v", m.GetIndex(), orderedIndicator)
		//continue
		return
	}

	//now incremented logIndicator == quorum
	if orderedIndicator != Quorum {
		log.Debugf("msg %v already broadcasted for ordering", m.GetIndex())
		return
	}*/

	if RSMResponsiveness{
		// @Gengrui Zhang
		//
		// Hint for developers:
		//
		// RSM. Responsiveness requires full phases.
		// Post and order phases already ensure that a quorum of servers
		// secured the unique order of the proposed transaction.
		//
		// Entering the commit phase, servers learn about there is a quorum
		// of servers have committed the proposed transaction at the secured
		// order in the ordering phase.
		//
		// It is possible to omit the commit phase if responsiveness can be
		// checked out of the consensus process. E.g., running a fact checker
		// that keeps track of server logs and conducts committing identical
		// logs.
		//
		return
	}

	// Leader commits and reply to clients
	ci := state.IncrementCommitIndex()

	log.Infof("New Commit: %v | LogIndex: %v | CommitIndex: %v\n", time.Now(), txLog.Index, ci)

	clientDock.Lock()
	delete(clientDock.m, txLog.Index)
	log.Debugf("Map size: %v | Cache of <%v, %v > was cleared", len(clientDock.m), txLog.Index, hex.EncodeToString(txLog.TxHash))
	clientDock.Unlock()

	err := notifyClient(clientDock.enc, txLog.TimeStamp[0], txLog.Index, txLog.TxHash)
	if err != nil {
		log.Errorf("notify client failed | err: %v", err)
		return
	}
}

func notifyClient(enc *gob.Encoder, ts, i int64, txHash []byte) error {
	clientConfirmation := clientConfirmEntry(ts, i, txHash)

	return enc.Encode(clientConfirmation)
}

func handleCommitPhaseServerConn(sConn *net.Conn, sid int){
	var commitPhaseErrorFlag = errors.New("commit phase")

	registerServer(sConn, CommitPhase, sid)

	log.Warningf("%v | new server registered | Id: %v -> Addr: %v\n", commitPhaseErrorFlag, sid, (*sConn).RemoteAddr())

	receiveCounter := int64(0)

	for {
		var m proto.WorkerOrderReply

		err := serverConnNav.n[CommitPhase][sid].dec.Decode(&m)

		counter := atomic.AddInt64(&receiveCounter, 1)

		//log.Debugf("server %v | len: %v | received: %+v", sid, unsafe.Sizeof(m), m)

		if err == io.EOF{
			log.Errorf("%v | server %v closed connection | err: %v", time.Now(), sid, err)
			break
		}

		if err != nil{
			log.Errorf("Gob Decode Err: %v | conn with ser: %v | remoteAddr: %v | Now # %v", err, sid, (*sConn).RemoteAddr(), counter)
			continue
		}

		if &m != nil {
			go asynHandleServerOrderReply(&m, sid)
		} else {
			log.Errorf("received message is nil")
		}
	}
}

func asynHandleServerOrderReply(m *proto.WorkerOrderReply, sid int) {

	if m == nil{
		log.Errorf("received WorkerOrderReply is empty")
		return
	}

	clientDock := clientConnNavigator[m.GetClientId()]

	if clientDock == nil {
		log.Errorf("corresponding client has not been registered | cid: %v", m.GetClientId())
		return
	}

	//asynchronousProcessLogReply(m, clientDock)

	log.Infof("WorkerOrderReply from Server %v | CId: %v | MsgIndex: %v | Sig: %v",
		sid, m.GetClientId(), m.GetIndex(), hex.EncodeToString(m.GetSignature()))

	clientDock.Lock()
	txLog, ok := clientDock.m[m.GetIndex()]
	if !ok {
		log.Debugf("no info in cache; consensus may already reached")
		return
	}

	committedIndicator := txLog.Committed

	if committedIndicator >= Quorum {
		log.Debugf("THIS SHOULD NEVER HAPPEN| commitIndicator: %v | Quorum: %v", committedIndicator, Quorum)
		delete(clientDock.m, m.GetIndex())
		clientDock.Unlock()
		//continue
		return
	}

	committedIndicator++
	clientDock.m[m.GetIndex()].Committed = committedIndicator
	aggregatedSigShares := append(clientDock.m[m.GetIndex()].CommitSigShares, m.GetSignature())
	clientDock.m[m.GetIndex()].CommitSigShares = aggregatedSigShares

	clientDock.Unlock()

	if committedIndicator < Quorum {
		log.Debugf("insufficient votes | TS: %v | committedIndicator: %v", m.GetIndex(), committedIndicator)
		//continue
		return
	}

	//now incremented logIndicator == quorum
	log.Debugf(" *** order reply votes suffice *** | TS: %v | committedIndicator: %v", m.GetIndex(), committedIndicator)

	sigThreshed, err := PenRecovery(aggregatedSigShares, txLog.CommitInfoHash)
	if err != nil{
		log.Errorf("PenRecovery failed | len(sigShares): %v | error: %v", len(aggregatedSigShares), err)
		return
	}

	//
	// Uncomment the following part if you need this info in cache.
	//
	// clientDock.Lock()
	// clientDock.m[m.GetIndex()].OrderedProofCert = sigThreshed
	// clientDock.Unlock()
	state.IncrementCommitIndex()

	commitEntry := commitPhaseEntry(m.GetIndex(), m.GetClientId(), sigThreshed)

	broadcast(commitEntry, CommitPhase)
	log.Debugf("=>=> commitEntry broadcast for msg %v", m.GetIndex())

	err = notifyClient(clientDock.enc, txLog.TimeStamp[0], txLog.Index, txLog.TxHash)
	if err != nil {
		log.Errorf("notify client failed | err: %v", err)
		return
	}
	log.Debugf("=>=> client notified for msg %v | client %v", m.GetIndex(), m.GetClientId())
}
