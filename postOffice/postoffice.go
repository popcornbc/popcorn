package postOffice

import "log"

const (
	PortExternalListener = iota
	PortInternalListenerPostPhase
	PortInternalListenerOrderPhase
	PortInternalListenerCommitPhase
	PortInternalElection
)

type PostOffice struct {
	myIndex	int
	serverLists		[]SerList
	Replication
}

type SerList struct {
	Index			int
	Ip	     		string
	Ports 			map[int]string
}
/*
 * The program should always call Init before the main function is executed
 * By calling NetworkHandler.Init the program initializes UDP connections
 *
 * The basic idea of network handler is to use TCP and UDP connections for log replication and
 * leader election/ view change, respectively
 *
 */

func (p *PostOffice) Init(index, maxCliConns, lid int, sl []SerList){
	p.myIndex = index
	p.serverLists = sl
	p.init(index, maxCliConns, lid, sl)
}

func (p *PostOffice) CloseAll(State int){

	for _, conn := range p.inListener {
		err := (*conn).Close()
		if err != nil {log.Printf("inListener.Close err: %v", err)}
	}

	for _, conn := range p.outListener {
		err := (*conn).Close()
		if err != nil {log.Printf("outListener.Close err: %v", err)}
	}
}
