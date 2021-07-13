package main

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"os"
	"popCli/proto"
	"strings"
)

func mockRandomString(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func mockTransactions(length int) string {
	return mockRandomString(length, charset)
}

//MACsSign is for signing hash of messages
//unless messages are smaller than their hashes
func MACsSign(clientId int, hash []byte) []byte{
	var macs = hmac.New(sha256.New, key[clientId])
	_, err := macs.Write(hash)

	if err != nil{
		log.Printf("MACs.Write failure | err: %v\n", err)
	}
	return macs.Sum(nil)
}

func formattingTxBeingSent(timestamp int64) *messager.MessageBank {

	prepareTx := mockTransactions(lenOfTransaction)
	macOfPreparedTx := MACsSign(ClientID, []byte(prepareTx))

	NewClientProposal := messager.ClientProposal{
		ClientID:	int64(ClientID),
		Timestamp:  timestamp,
		Tx:        prepareTx,
		Macs:      macOfPreparedTx,
	}

	NewMessage := &messager.MessageBank{
		CmuType:	messager.CommunicationType_Type_Client_To_Cluster,
		ClientProposal: &NewClientProposal,
	}

	return NewMessage
}

func loadMessageQueue(){

	for i := 0; i < numOfMsgToBeSent; i++{

/*		msg, err := serialization()
		if err != nil {
			log.Fatal("marshaling error: ", err)
		}*/

		//msg = append(msg, '|')

		//gob.NewEncoder(formattingTxBeingSent(int32(i)))
		MQ <- *formattingTxBeingSent(int64(i))

		//fmt.Println(s)
		if i == 1 {
			//have a peak at msg and its length
			log.Infof("message going to be sent: %+v\n",
				formattingTxBeingSent(int64(i)))
		}
	}
	log.Infof("Message Q loading completed | numOfMsgToBeSent: %d\n", numOfMsgToBeSent)
}


func parseClientFile(ClientID int) [][]byte{
	var clientKeys [][]byte

	c, err := os.Open("clients.config")
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(c)
	scanner.Split(bufio.ScanLines)

	var r []string
	for scanner.Scan(){
		r = append(r, scanner.Text())
	}

	err = c.Close()
	if err != nil {
		log.Errorf("Close clients.config failed | err: %v\n", err)
	}

	if ClientID > len(r) {
		panic(errors.New("ClientID out of bound"))
	}

	for _, row := range r{
		row := strings.Split(row, " ")

		clientKeys = append(clientKeys, []byte(row[0]))
	}
	return clientKeys
}

func parseServerFile(){
	c, err := os.Open("servers.config")
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(c)
	scanner.Split(bufio.ScanLines)

	var r []string
	for scanner.Scan(){
		r = append(r, scanner.Text())
	}

	err = c.Close()
	if err != nil {
		log.Errorf("Close servers.config failed | err: %v\n", err)
	}

	for _, row := range r{
		row := strings.Split(row, " ")

		ServerList = append(ServerList, row[0])
	}
}

func serialization(m *messager.MessageBank) ([]byte, error){
	return proto.Marshal(m)
	//return json.Marshal(m)
}

func deserialization(b *[]byte, m *messager.MessageBank) error{
	return proto.Unmarshal(*b, m)
	//return json.Unmarshal(*b, m)
}
