package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"github.com/dedis/kyber/share"
	"github.com/dedis/kyber/sign/bls"
	kyberOldScalar "go.dedis.ch/kyber"
	"log"
	"popcorn/penKeyGen"
	messager "popcorn/proto"
)

func serialization(m *messager.MessageBank) ([]byte, error){
	//return proto.Marshal(m)
	return json.Marshal(m)
}

func deserialization(b *[]byte, m *messager.MessageBank) error{
	//return proto.Unmarshal(*b, m)
	return json.Unmarshal(*b, m)
}


func fetchKeyGen(){
	PublicPoly, PrivateShare = FetchPublicPolyAndPrivateKeyShare()
	//v1, v2 := PublicPoly.Info()
	//fmt.Printf("%v | %v\n", v1, v2)

}

func FetchPublicPolyAndPrivateKeyShare() (*share.PubPoly, *share.PriShare){

	var prosecutorSecrets [] kyberOldScalar.Scalar

	for i := 0; i < Threshold; i++ {
		mySecret := ServerSecrets[i]
		secret := suite.G1().Scalar().SetBytes(mySecret)
		prosecutorSecrets = append(prosecutorSecrets, secret)
	}

	//fmt.Printf("secret: %v\n", prosecutorSecrets)
	PriPoly := share.NewProsecutorPriPoly(suite.G2(), Threshold, prosecutorSecrets)
	PrivatePoly = PriPoly
	//fmt.Println("PriPoly:", PriPoly.String())
	return PriPoly.Commit(suite.G2().Point().Base()), PriPoly.Shares(NumOfServers)[ThisServerID]
}

func getDigest(x []byte) []byte{
	r := sha256.Sum256(x)
	return r[:]
}

func ValidateMACs(clientID int64, rMsg string, rMACs []byte) bool{

	key := ClientSecrets[clientID]
	var macs = hmac.New(sha256.New, key)
	_, err := macs.Write([]byte(rMsg))

	if err != nil{
		log.Printf("MACs.Write failure | err: %v\n", err)
	}
	verifyingMACs := macs.Sum(nil)

	return hmac.Equal(verifyingMACs, rMACs)
}

func signMsgDigest(tx string)([]byte, []byte, error){
	digest := getDigest([]byte(tx))
	sig, err := PenSign(digest[:])
	return digest[:], sig, err
}

func PenSign(msg []byte) ([]byte, error){
	return penKeyGen.Sign(suite, PrivateShare, msg)
}

func PenRecovery(sigShares [][]byte, msg []byte) ([]byte, error){
	sig, err := penKeyGen.Recover(suite, PublicPoly, msg, sigShares, Threshold, NumOfServers)
	return sig, err
}

func PenVerify(msg, sig []byte) error {
	return bls.Verify(suite, PublicPoly.Commit(), msg, sig)
}