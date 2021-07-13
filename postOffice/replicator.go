package postOffice

import (
	"log"
	"net"
	"sync"
	"time"
)

type Replication struct {
	numSerConns		int
	numMaxCliConns	int

	inListener		map[string]*net.Listener
	muInListener 	sync.RWMutex

	outListener		map[string]*net.Listener
	muOutListener 	sync.RWMutex

	SerConns		chan *net.Conn
	CliConns		chan *net.Conn

	DialConn		*net.Conn
}


func (r *Replication) init(index, maxCliConns, leaderId int, sl []SerList)  {
	//inLnAddr 		:= sl[index].Ip+":"+ sl[index].InLnPort
	outListenerAddr  := sl[index].Ip+":"+ sl[index].Ports[PortExternalListener]

	r.numSerConns	 = len(sl)
	r.numMaxCliConns = maxCliConns

	r.SerConns 		 = make(chan *net.Conn, r.numSerConns)
	r.CliConns 		 = make(chan *net.Conn, r.numMaxCliConns)

	r.inListener 	= make(map[string]*net.Listener)
	r.outListener 	= make(map[string]*net.Listener)

	r.outListener[outListenerAddr]= startListener(outListenerAddr)

	go r.ListeningOuterConnections()

	//Let us leave this ugly initialization for now
	if index != leaderId {
		r.initDialConn(sl[leaderId])
		return
	}

	log.Println("This is leader server")
}

func (r *Replication)initDialConn(leader SerList){
	var e error
	r.DialConn, e = dialLeaderConn(leader)
	if e != nil{
		panic("leader is down")
	}
	log.Printf("dialed to leader | myC: %v | leaderC: %v \n",
		(*r.DialConn).LocalAddr(), (*r.DialConn).RemoteAddr())
}

func dialLeaderConn(leader SerList) (*net.Conn, error){

	//retry if leader's not up yet

	toWhere := leader.Ip+":"+leader.Ports[PortInternalListenerPostPhase]
	var e error
	maxTry := 5
	for i := 0; i < maxTry; i ++ {
		//conn, err := net.DialTCP("tcp4", loTcpAddr, reTcpAddr)
		conn, err := net.Dial("tcp4", toWhere)
		if err != nil{
			log.Printf("Dial Leader failed | err: %v | maxTry: %v | retry: %vth\n", err, maxTry, i)
			time.Sleep(1 *time.Second)
			e = err
			continue
		}
		return &conn, nil
	}

	return nil, e
}

func startListener(listenerAddr string) *net.Listener{
	ln, err := net.Listen("tcp4", listenerAddr)
	if err != nil{
		panic(err)
	}
	return &ln
}

func (r *Replication) ListeningOuterConnections(){

	go func() {
		r.muOutListener.Lock()

		for _, listener := range r.outListener{
			clientConn, err := (*listener).Accept()
			if err != nil{
				log.Println(err)
				continue
			}

			r.CliConns <- &clientConn
			log.Printf("POST OFFICE | New Client Connection | %v", clientConn.RemoteAddr())
		}

		r.muOutListener.Unlock()
	}()
}