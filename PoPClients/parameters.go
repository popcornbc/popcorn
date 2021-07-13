package main

import (
	"flag"
)


const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var lenOfTransaction = 10
var numOfMsgToBeSent = 100000 //100,000

/************* this part is for MACs sign ****************/

const (
	REGULAR = iota
	INTENSIVE //for future implementation
)

var key [][]byte

var ClientID 		int
var NumOfAsyn		int
var NumOfSender 	int
var SendModel 		int
var TestingModel = true
func loadParametersFromCommandLine(){

	flag.IntVar(&ClientID, "id", 0, "this client Id")
	flag.IntVar(&NumOfAsyn, "asyn", 10, "number of asyn")
	flag.IntVar(&NumOfSender, "senders", 1, "number of senders")
	flag.IntVar(&SendModel, "send model", REGULAR, "0 for regular; 1 for intensive")
	flag.BoolVar(&TestingModel, "testing", false, "true for testing")
	flag.Parse()
}