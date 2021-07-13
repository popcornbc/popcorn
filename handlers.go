package main

import (
	"encoding/gob"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"sync"
	"time"
)

//
// clientConnNavigator can be changed to slice instead of an array
//
var clientConnNavigator [100]*clientConnDock
var muClientConnNavigator sync.RWMutex

type clientConnDock struct{
	sync.RWMutex
	clientConn *net.Conn
	m map[int64]*TXLog
	enc *gob.Encoder
	dec *gob.Decoder
}

var serverConnNavigator = make([]*serverConnDock, NumOfServers)
var muServerConnNavigator sync.RWMutex

type serverConnDock struct {
	sync.RWMutex
	serverConn *net.Conn
	serverId int
	enc *gob.Encoder
	dec *gob.Decoder
}

func broadcast(e interface{}){

	for i := 0; i < len(serverConnNavigator); i ++ {
		if ThisServerID == i {
			continue
		}
		go func(i int) {

			err := serverConnNavigator[i].enc.Encode(e)
			if err != nil{
				switch err {
				case io.EOF:
					log.Error("server %v closed connection | err: %v", serverConnNavigator[i].serverId, err)
					break
				default:
					log.Errorf("sent to server %v failed | err: %v", serverConnNavigator[i].serverId, err)
				}
			}
		}(i)
	}
}

type TXLog struct{
	Index			int64
	//ClientID 		int64
	TimeStamp 		int64
	TxHash 			[]byte
	LogIndicator 	int
	OrderIndicator 	int
	CommitIndicator int
	SigShares 		[][]byte
	Committed 		bool
}


// recvDialConn is enabled when server is follower
// This function receives packets from leader

func statusReport(){
	for{
		rc := CommitIndex

		time.Sleep(1 * time.Second)

		fmt.Printf("=>=>=>=>=>=>=>=>=>=>=>=>=>=>\n" +
			"> LogIndex: %v | CommitIndex: %v | Throughput: %v;\n",
			LogIndex, CommitIndex, int(CommitIndex-rc) * BatchSize)
	}
}

func start(){

	go clientCommunication()
	go serverCommunication()
	go statusReport()
}
