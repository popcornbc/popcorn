package main

import (
	"encoding/gob"
	"fmt"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"net"
	"path"
	messager "popCli/proto"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var MQ = make(chan messager.MessageBank, 1000000)

var asynChan chan int

var wg sync.WaitGroup
var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

var ServerList	[]string
var SerConns	[]net.Conn
var confirmIndex int64

func init(){

	loadParametersFromCommandLine()

	asynChan = make(chan int, NumOfAsyn)

	key = parseClientFile(ClientID)

	parseServerFile()

	log.SetReportCaller(true)
	ProsecutorLogFormat := &log.TextFormatter{
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			//return fmt.Sprintf("%s()", f.Function), fmt.Sprintf("%s:%d", filename, f.Line)
			return "", fmt.Sprintf("%s:%d", filename, f.Line)
		},
	}
	log.SetFormatter(ProsecutorLogFormat)
}

func closeConns(conns *[]net.Conn){
	for i := 0; i < len(*conns); i ++ {
		err := (*conns)[i].Close()
		if err != nil{
			log.Errorln(err)
		}
	}
}

type serInfo struct{
	serId int
	enc gob.Encoder
	dec gob.Decoder
}

func main(){
	runtime.GOMAXPROCS(runtime.NumCPU())

	loadMessageQueue()

	log.Infof("%v\n", ServerList)

	SerConns = make([]net.Conn, len(ServerList))
	defer closeConns(&SerConns)

	var serverConnections []serInfo

	for i, ad := range ServerList {
		c, err := net.Dial("tcp4", ad)

		if err != nil{
			panic(err)
		}

		ac := serInfo{
			serId: i,
			enc:   *gob.NewEncoder(c),
			dec:   *gob.NewDecoder(c),
		}
		//go connections(c, i)
		serverConnections = append(serverConnections, ac)
	}

	asynChan <- 1
	//<- MQ
	go connSend(&serverConnections)
	go connRec(&serverConnections)

	wg.Add(1)
	wg.Wait()
}


func connSend(c *[]serInfo){
	for {
		p := <- MQ

		for i := 0; i < len(*c); i ++ {
			err := (*c)[i].enc.Encode(p)
			if err != nil{
				log.Errorln(err)
				break
			}
		}

		//log.Infof("Sent")
		asynChan <- 1
	}
}

func connRec(c *[]serInfo){

	for i := 0; i < len(*c); i ++ {
		go func(c serInfo, n int) {
			for {
				var m messager.MessageBank

				err := c.dec.Decode(&m)
				if err != nil{
					log.Errorln(err)
					break
				}

				//log.Infof("received %v | %v", c.serId, m)

				//m.GetClientConfirmation().GetTimestamp()
				counter := atomic.AddInt64(&confirmIndex, 1)
				if counter % int64(n) == 0 {
					<- asynChan
					//time.Sleep(1 * time.Second)
				}

				if counter % int64(1000*n) == 0{
					log.Infof("Recv 1000 tx | Client: %v | %v ", ClientID, time.Now())
				}
			}
		}((*c)[i], len(*c))
	}
}
