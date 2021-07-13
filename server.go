package main

import (
	"flag"
	"fmt"
	"popcorn/postOffice"
	messager "popcorn/proto"
	"runtime"
	"sync"
)

var postman postOffice.PostOffice

type Pipes [10]chan *messager.MessageBank

func (p *Pipes) Init(capacityOfPipes int){
	for i :=0; i < len(p); i ++{
		p[i] = make(chan *messager.MessageBank, capacityOfPipes)
		//p.Append(pipe)
	}
	return
}

var CurrentLeaderID int

func init(){

	flag.Parse()

	newParse(NumOfServers, MaxClientConnection)

	CurrentLeaderID = 0

	postman.Init(ThisServerID, MaxClientConnection, CurrentLeaderID, ServerList)

	fetchKeyGen()

	setLogger()
	fmt.Printf("On board information checkbook\n|len of MQ: %10v|\n", LenOfMessageQ)
}

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())

	defer postman.CloseAll(State)

	go start()

	var wg sync.WaitGroup
	wg.Add(1)

	wg.Wait()
}
