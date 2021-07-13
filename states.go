package main

const (
	LEADER = iota
	NOMINEE
	STARLET
	WORKER
)

var (
	LogIndex 				 	int64
	CommitIndex               	int64
	packetReceivedFromServers	int64
)