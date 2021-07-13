package main

import (
	"flag"
	"github.com/dedis/kyber/pairing/bn256"
	"github.com/dedis/kyber/share"
	"popcorn/postOffice"
)

var ServerList []postOffice.SerList

var ServerSecrets [][]byte
var ClientSecrets [][]byte

var PublicPoly *share.PubPoly
var PrivatePoly *share.PriPoly

var PrivateShare *share.PriShare
var suite = bn256.NewSuite()


var Quorum = 3

const MaxClientConnection = 100

//below parameters initialized through func loadParametersFromCommandLine()
var (
	//parameters related to performance

	BatchSize = *flag.Int("batch", 1,"BatchSize")
	LenOfMessageQ = *flag.Int("MQ", 65536,"LenOfMessageQ")

	LogLevel = *flag.Int("log", WarnLevel,"0: PanicLevel | 1: FatalLevel | " +
		"2 : ErrorLevel | 3: WarnLevel | 4: InfoLevel | 5: DebugLevel")
	NumOfServers = *flag.Int("n", 3,"# of servers")
	Threshold = *flag.Int("th", 2, "threshold")
	ThisServerID = *flag.Int("id", 0, "serverID")

	ThreePhaseCommit = *flag.Bool("3pc", false,"3PC")

	State = *flag.Int("state", LEADER,
		"0 : Leader | 1 : Nominee | 2 : Starlet | 3 : Worker")
)

const (
	PanicLevel = iota 	//0
	FatalLevel 			//1
	ErrorLevel 			//2
	WarnLevel 			//3
	InfoLevel 			//4
	DebugLevel 			//5
	TraceLevel 			//6
)