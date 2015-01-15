package reprovide

import (
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	blocks "github.com/jbenet/go-ipfs/blocks/blockstore"
	routing "github.com/jbenet/go-ipfs/routing"
	"github.com/jbenet/go-ipfs/util/eventlog"
)

var log = eventlog.Logger("reprovider")

type Reprovider struct {
	// The routing system to provide values through
	rsys routing.IpfsRouting

	// The backing store for blocks to be provided
	bstore blocks.Blockstore
}

func NewReprovider(rsys routing.IpfsRouting, bstore blocks.Blockstore) *Reprovider {
	return &Reprovider{
		rsys:   rsys,
		bstore: bstore,
	}
}

func (rp *Reprovider) ProvideEvery(ctx context.Context, tick time.Duration) {
	after := time.After(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-after:
			rp.Reprovide(ctx)
			after = time.After(tick)
		}
	}
}

func (rp *Reprovider) Reprovide(ctx context.Context) {
	keychan, err := rp.bstore.AllKeysChan(ctx, 0, 1<<16)
	if err != nil {
		log.Errorf("Failed to get key chan from blockstore: %s", err)
		return
	}
	for k := range keychan {
		err := rp.rsys.Provide(ctx, k)
		if err != nil {
			log.Errorf("Failed to provide key: %s, %s", k, err)
		}
	}
}
