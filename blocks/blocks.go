package blocks

import (
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	u "github.com/jbenet/go-ipfs/util"
)

// Block is a singular block of data in ipfs
type Block struct {
	Multihash mh.Multihash
	Data      []byte
}

// NewBlock creates a Block object from opaque data. It will hash the data.
func NewBlock(data []byte) (*Block, error) {
	h, err := u.Hash(data)
	if err != nil {
		return nil, err
	}
	return &Block{Data: data, Multihash: h}, nil
}

// Key returns the block's Multihash as a Key value.
func (b *Block) Key() u.Key {
	return u.Key(b.Multihash)
}
