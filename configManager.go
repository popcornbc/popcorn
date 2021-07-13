package main

import (
	"bufio"
	"errors"
	log "github.com/sirupsen/logrus"
	"os"
	"popcorn/postOffice"
	"strconv"
	"strings"
)

func newParse(n, m int){
	ServerList = parseServerFile(n)
	ClientSecrets = parseClientFile(m)
}

func parseServerFile(n int) []postOffice.SerList{

	var fileRows []string
	var sl []postOffice.SerList

	s, err := os.Open("servers.config")
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(s)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan(){
		fileRows = append(fileRows, scanner.Text())
	}

	err = s.Close()
	if err != nil {
		log.Errorf("close fileServer failed | err: %v\n", err)
	}

	if len(fileRows) != n {
		log.Errorf("Going to panic | fileRows: %v | n: %v", len(fileRows), n)
		panic(errors.New("number of servers in config file does not match with provided $n$"))
	}

	for i:=0 ; i < len(fileRows); i ++{
		var singleSL postOffice.SerList

		row := strings.Split(fileRows[i], " ")

		i, err := strconv.Atoi(row[0])
		if err != nil {
			panic(err)
		}
		singleSL.Index = i
		//row[1]: ip
		singleSL.Ip = row[1]

		//row[2]: secret
		ServerSecrets = append(ServerSecrets, []byte(row[2]))

		singleSL.Ports = make(map[int]string)

		singleSL.Ports[postOffice.PortExternalListener] = row[3]

		singleSL.Ports[postOffice.PortInternalListenerPostPhase] = row[4]

		singleSL.Ports[postOffice.PortInternalListenerOrderPhase] = row[5]

		singleSL.Ports[postOffice.PortInternalListenerCommitPhase] = row[6]

		singleSL.Ports[postOffice.PortInternalElection] = row[7]

		sl = append(sl, singleSL)
	}

	return sl
}

func parseClientFile(maxCliConns int) [][]byte{
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

	if len(r) > maxCliConns {
		log.Errorf("Too many clients")
	}
	for _, row := range r{
		row := strings.Split(row, " ")

		clientKeys = append(clientKeys, []byte(row[0]))
	}
	return clientKeys
}