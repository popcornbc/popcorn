package penKeyGen

import (
	"bytes"
	"encoding/binary"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/share"
	"github.com/dedis/kyber/sign/bls"
)

type SigShare []byte

// Index returns the index i of the TBLS share Si.
func (s SigShare) Index() (int, error) {
	var index uint16
	buf := bytes.NewReader(s)
	err := binary.Read(buf, binary.BigEndian, &index)
	if err != nil {
		return -1, err
	}
	return int(index), nil
}

// Value returns the value v of the TBLS share Si.
func (s *SigShare) Value() []byte {
	return []byte(*s)[2:]
}

// Sign creates a threshold BLS signature Si = xi * H(m) on the given message m
// using the provided secret key share xi.
func Sign(suite pairing.Suite, private *share.PriShare, msg []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, uint16(private.I)); err != nil {
		return nil, err
	}
	s, err := bls.Sign(suite, private.V, msg)
	if err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, s); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Recover(suite pairing.Suite, public *share.PubPoly, msg []byte, sigs [][]byte, t, n int) ([]byte, error) {
	pubShares := make([]*share.PubShare, 0)
	for _, sig := range sigs {
		s := SigShare(sig)
		i, err := s.Index()
		if err != nil {
			return nil, err
		}
		if err = bls.Verify(suite, public.Eval(i).V, msg, s.Value()); err != nil {
			return nil, err
		}
		point := suite.G1().Point()
		if err := point.UnmarshalBinary(s.Value()); err != nil {
			return nil, err
		}
		pubShares = append(pubShares, &share.PubShare{I: i, V: point})
		if len(pubShares) >= t {
			break
		}
	}
	commit, err := share.RecoverCommit(suite.G1(), pubShares, t, n)
	if err != nil {
		return nil, err
	}
	sig, err := commit.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return sig, nil
}
