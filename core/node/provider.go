package node

import (
	"context"
	"time"

	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/provider"
	q "github.com/ipfs/go-ipfs/provider/queue"
	"github.com/ipfs/go-ipfs/provider/simple"
	"github.com/ipfs/go-ipfs/repo"

	"github.com/libp2p/go-libp2p-core/routing"
	"go.uber.org/fx"
)

const DefaultReprovideFrequency = time.Hour * 12

// SIMPLE

// ProviderQueue creates new datastore backed provider queue
func ProviderQueue(mctx helpers.MetricsCtx, lc fx.Lifecycle, repo repo.Repo) (*q.Queue, error) {
	return q.NewQueue(helpers.LifecycleCtx(mctx, lc), "provider-v1", repo.Datastore())
}

// SimpleProvider creates new record provider
func SimpleProvider(mctx helpers.MetricsCtx, lc fx.Lifecycle, queue *q.Queue, rt routing.Routing) provider.Provider {
	return simple.NewProvider(helpers.LifecycleCtx(mctx, lc), queue, rt)
}

// SimpleReprovider creates new reprovider
func SimpleReprovider(reproviderInterval time.Duration) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, rt routing.Routing, keyProvider simple.KeyChanFunc) (provider.Reprovider, error) {
		return simple.NewReprovider(helpers.LifecycleCtx(mctx, lc), reproviderInterval, rt, keyProvider), nil
	}
}

// SimpleProviderSys creates new provider system
func SimpleProviderSys(isOnline bool) interface{} {
	return func(lc fx.Lifecycle, p provider.Provider, r provider.Reprovider) provider.System {
		sys := provider.NewSystem(p, r)

		if isOnline {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					sys.Run()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					return sys.Close()
				},
			})
		}

		return sys
	}
}
