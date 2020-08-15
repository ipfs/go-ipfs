package cidv0v1

import (
	"context"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	bs "github.com/ipfs/go-ipfs-blockstore"
	mh "github.com/multiformats/go-multihash"
)

type blockstore struct {
	bs.Blockstore
}

func NewBlockstore(b bs.Blockstore) bs.Blockstore {
	return &blockstore{b}
}

func (b *blockstore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	have, err := b.Blockstore.Has(ctx, c)
	if have || err != nil {
		return have, err
	}
	c1 := tryOtherCidVersion(c)
	if !c1.Defined() {
		return false, nil
	}
	return b.Blockstore.Has(ctx, c1)
}

func (b *blockstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	block, err := b.Blockstore.Get(ctx, c)
	if err == nil {
		return block, nil
	}
	if err != bs.ErrNotFound {
		return nil, err
	}
	c1 := tryOtherCidVersion(c)
	if !c1.Defined() {
		return nil, bs.ErrNotFound
	}
	block, err = b.Blockstore.Get(ctx, c1)
	if err != nil {
		return nil, err
	}
	// modify block so it has the original CID
	block, err = blocks.NewBlockWithCid(block.RawData(), c)
	if err != nil {
		return nil, err
	}
	// insert the block with the original CID to avoid problems
	// with pinning
	err = b.Blockstore.Put(ctx, block)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (b *blockstore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	size, err := b.Blockstore.GetSize(ctx, c)
	if err == nil {
		return size, nil
	}
	if err != bs.ErrNotFound {
		return -1, err
	}
	c1 := tryOtherCidVersion(c)
	if !c1.Defined() {
		return -1, bs.ErrNotFound
	}
	return b.Blockstore.GetSize(ctx, c1)
}

func tryOtherCidVersion(c cid.Cid) cid.Cid {
	prefix := c.Prefix()
	if prefix.Codec != cid.DagProtobuf || prefix.MhType != mh.SHA2_256 || prefix.MhLength != 32 {
		return cid.Undef
	}
	var c1 cid.Cid
	if prefix.Version == 0 {
		c1 = cid.NewCidV1(cid.DagProtobuf, c.Hash())
	} else {
		c1 = cid.NewCidV0(c.Hash())
	}
	return c1
}
