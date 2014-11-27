// package blockservice implements a BlockService interface that provides
// a single GetBlock/AddBlock interface that seamlessly retrieves data either
// locally or from a remote peer through the exchange.
package blockservice

import (
	"errors"
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	blocks "github.com/jbenet/go-ipfs/blocks"
	"github.com/jbenet/go-ipfs/blocks/blockstore"
	exchange "github.com/jbenet/go-ipfs/exchange"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("blockservice")
var ErrNotFound = errors.New("blockservice: key not found")

// BlockService is a hybrid block datastore. It stores data in a local
// datastore and may retrieve data from a remote Exchange.
// It uses an internal `datastore.Datastore` instance to store values.
type BlockService struct {
	// TODO don't expose underlying impl details
	Blockstore blockstore.Blockstore
	Exchange   exchange.Interface
}

// NewBlockService creates a BlockService with given datastore instance.
func New(bs blockstore.Blockstore, rem exchange.Interface) (*BlockService, error) {
	if bs == nil {
		return nil, fmt.Errorf("BlockService requires valid blockstore")
	}
	if rem == nil {
		log.Warning("blockservice running in local (offline) mode.")
	}
	return &BlockService{Blockstore: bs, Exchange: rem}, nil
}

// AddBlock adds a particular block to the service, Putting it into the datastore.
// TODO pass a context into this if the remote.HasBlock is going to remain here.
func (s *BlockService) AddBlock(b *blocks.Block) (u.Key, error) {
	err := s.Exchange.HasBlock(context.TODO(), b)
	return b.Key(), err
}

// GetBlock retrieves a particular block from the service,
// Getting it from the datastore using the key (hash).
func (s *BlockService) GetBlock(ctx context.Context, k u.Key) (*blocks.Block, error) {
	blk, err := s.Exchange.GetBlock(ctx, k)
	if err != nil {
		return nil, ErrNotFound
	}
	return blk, nil
}

// GetBlocks gets a list of blocks asynchronously and returns through
// the returned channel.
// NB: No guarantees are made about order.
func (s *BlockService) GetBlocks(ctx context.Context, ks []u.Key) <-chan *blocks.Block {
	out := make(chan *blocks.Block, 0)
	go func() {
		defer close(out)
		var misses []u.Key
		for _, k := range ks {
			hit, err := s.Blockstore.Get(k)
			if err != nil {
				misses = append(misses, k)
				continue
			}
			log.Debug("Blockservice: Got data in datastore.")
			select {
			case out <- hit:
			case <-ctx.Done():
				return
			}
		}

		rblocks, err := s.Exchange.GetBlocks(ctx, misses)
		if err != nil {
			log.Errorf("Error with GetBlocks: %s", err)
			return
		}

		for b := range rblocks {
			select {
			case out <- b:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// DeleteBlock deletes a block in the blockservice from the datastore
func (s *BlockService) DeleteBlock(k u.Key) error {
	return s.Blockstore.DeleteBlock(k)
}
