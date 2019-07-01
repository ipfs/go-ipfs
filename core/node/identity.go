package node

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	mh "github.com/multiformats/go-multihash"
)

func PeerID(id peer.ID) func() peer.ID {
	return func() peer.ID {
		return id
	}
}

func RandomPeerID() (peer.ID, error) {
	b, err := ioutil.ReadAll(io.LimitReader(rand.Reader, 32))
	if err != nil {
		return "", err
	}
	hash, err := mh.Sum(b, mh.SHA2_256, -1)
	if err != nil {
		return "", err
	}
	return peer.ID(hash), nil
}

// PrivateKey loads the private key from config
func PrivateKey(sk crypto.PrivKey) func(id peer.ID) (crypto.PrivKey, error) {
	return func(id peer.ID) (crypto.PrivKey, error) {
		id2, err := peer.IDFromPrivateKey(sk)
		if err != nil {
			return nil, err
		}

		if id2 != id {
			return nil, fmt.Errorf("private key in config does not match id: %s != %s", id, id2)
		}
		return sk, nil
	}
}
