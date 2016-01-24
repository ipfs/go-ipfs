// package blockservice implements a BlockService interface that provides
// a single GetBlock/AddBlock interface that seamlessly retrieves data either
// locally or from a remote peer through the exchange.
package blockservice

import (
	"errors"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	blocks "github.com/ipfs/go-ipfs/blocks"
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	exchange "github.com/ipfs/go-ipfs/exchange"
	logging "github.com/ipfs/go-ipfs/vendor/QmQg1J6vikuXF9oDvm4wpdeAUvvkVEKW1EYDw9HhTMnP2b/go-log"
)

var log = logging.Logger("blockservice")

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
func New(bs blockstore.Blockstore, rem exchange.Interface) *BlockService {
	if rem == nil {
		log.Warning("blockservice running in local (offline) mode.")
	}

	return &BlockService{
		Blockstore: bs,
		Exchange:   rem,
	}
}

// AddBlock adds a particular block to the service, Putting it into the datastore.
// TODO pass a context into this if the remote.HasBlock is going to remain here.
func (s *BlockService) AddBlock(b *blocks.Block) (key.Key, error) {
	k := b.Key()
	err := s.Blockstore.Put(b)
	if err != nil {
		return k, err
	}
	if err := s.Exchange.HasBlock(b); err != nil {
		return "", errors.New("blockservice is closed")
	}
	return k, nil
}

func (s *BlockService) AddBlocks(bs []*blocks.Block) ([]key.Key, error) {
	err := s.Blockstore.PutMany(bs)
	if err != nil {
		return nil, err
	}

	var ks []key.Key
	for _, b := range bs {
		if err := s.Exchange.HasBlock(b); err != nil {
			return nil, errors.New("blockservice is closed")
		}
		ks = append(ks, b.Key())
	}
	return ks, nil
}

// GetBlock retrieves a particular block from the service,
// Getting it from the datastore using the key (hash).
func (s *BlockService) GetBlock(ctx context.Context, k key.Key) (*blocks.Block, error) {
	log.Debugf("BlockService GetBlock: '%s'", k)
	block, err := s.Blockstore.Get(k)
	if err == nil {
		return block, nil
	}

	if err == blockstore.ErrNotFound && s.Exchange != nil {
		// TODO be careful checking ErrNotFound. If the underlying
		// implementation changes, this will break.
		log.Debug("Blockservice: Searching bitswap.")
		blk, err := s.Exchange.GetBlock(ctx, k)
		if err != nil {
			if err == blockstore.ErrNotFound {
				return nil, ErrNotFound
			}
			return nil, err
		}
		return blk, nil
	}

	log.Debug("Blockservice GetBlock: Not found.")
	if err == blockstore.ErrNotFound {
		return nil, ErrNotFound
	}

	return nil, err
}

// GetBlocks gets a list of blocks asynchronously and returns through
// the returned channel.
// NB: No guarantees are made about order.
func (s *BlockService) GetBlocks(ctx context.Context, ks []key.Key) <-chan *blocks.Block {
	out := make(chan *blocks.Block, 0)
	go func() {
		defer close(out)
		var misses []key.Key
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
			log.Debugf("Error with GetBlocks: %s", err)
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
func (s *BlockService) DeleteBlock(k key.Key) error {
	return s.Blockstore.DeleteBlock(k)
}

func (s *BlockService) Close() error {
	defer log.Debug("blockservice is shutting down...")
	return s.Exchange.Close()
}
