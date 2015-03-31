package record

import (
	"bytes"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	pb "github.com/ipfs/go-ipfs/routing/dht/pb"
	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"
	u "github.com/ipfs/go-ipfs/util"
)

var log = eventlog.Logger("routing/record")

// MakePutRecord creates and signs a dht record for the given key/value pair
func MakePutRecord(sk ci.PrivKey, key u.Key, value []byte, sign bool) (*pb.Record, error) {
	record := new(pb.Record)

	record.Key = proto.String(string(key))
	record.Value = value

	pkh, err := sk.GetPublic().Hash()
	if err != nil {
		return nil, err
	}

	record.Author = proto.String(string(pkh))
	if sign {
		blob := RecordBlobForSig(record)

		sig, err := sk.Sign(blob)
		if err != nil {
			return nil, err
		}

		record.Signature = sig
	}
	return record, nil
}

// RecordBlobForSig returns the blob protected by the record signature
func RecordBlobForSig(r *pb.Record) []byte {
	k := []byte(r.GetKey())
	v := []byte(r.GetValue())
	a := []byte(r.GetAuthor())
	return bytes.Join([][]byte{k, v, a}, []byte{})
}
