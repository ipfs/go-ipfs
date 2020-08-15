package verifbs

import (
	"context"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-verifcid"
)

type VerifBSGC struct {
	bstore.GCBlockstore
}

func (bs *VerifBSGC) Put(ctx context.Context, b blocks.Block) error {
	if err := verifcid.ValidateCid(b.Cid()); err != nil {
		return err
	}
	return bs.GCBlockstore.Put(ctx, b)
}

func (bs *VerifBSGC) PutMany(ctx context.Context, blks []blocks.Block) error {
	for _, b := range blks {
		if err := verifcid.ValidateCid(b.Cid()); err != nil {
			return err
		}
	}
	return bs.GCBlockstore.PutMany(ctx, blks)
}

func (bs *VerifBSGC) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	if err := verifcid.ValidateCid(c); err != nil {
		return nil, err
	}
	return bs.GCBlockstore.Get(ctx, c)
}

type VerifBS struct {
	bstore.Blockstore
}

func (bs *VerifBS) Put(ctx context.Context, b blocks.Block) error {
	if err := verifcid.ValidateCid(b.Cid()); err != nil {
		return err
	}
	return bs.Blockstore.Put(ctx, b)
}

func (bs *VerifBS) PutMany(ctx context.Context, blks []blocks.Block) error {
	for _, b := range blks {
		if err := verifcid.ValidateCid(b.Cid()); err != nil {
			return err
		}
	}
	return bs.Blockstore.PutMany(ctx, blks)
}

func (bs *VerifBS) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	if err := verifcid.ValidateCid(c); err != nil {
		return nil, err
	}
	return bs.Blockstore.Get(ctx, c)
}
