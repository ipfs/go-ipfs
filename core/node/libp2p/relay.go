package libp2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p"
	circuit "github.com/libp2p/go-libp2p-circuit"
	coredisc "github.com/libp2p/go-libp2p-core/discovery"
	routing "github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	relay "github.com/libp2p/go-libp2p/p2p/host/relay"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/node/helpers"
)

func Relay(disable, enableHop bool) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		if disable {
			// Enabled by default.
			opts.Opts = append(opts.Opts, libp2p.DisableRelay())
		} else {
			relayOpts := []circuit.RelayOpt{circuit.OptDiscovery}
			if enableHop {
				relayOpts = append(relayOpts, circuit.OptHop)
			}
			opts.Opts = append(opts.Opts, libp2p.EnableRelay(relayOpts...))
		}
		return
	}
}

// TODO: should be use baseRouting or can we use higher level router here?
func Discovery(router BaseIpfsRouting) (coredisc.Discovery, error) {
	crouter, ok := router.(routing.ContentRouting)
	if !ok {
		return nil, fmt.Errorf("no suitable routing for discovery")
	}

	return discovery.NewRoutingDiscovery(crouter), nil
}

// NOTE: only set when relayHop is set
func AdvertiseRelay(mctx helpers.MetricsCtx, d coredisc.Discovery) {
	relay.Advertise(mctx, d)
}

// NOTE: only set when relayHop is not set
func AutoRelay(mctx helpers.MetricsCtx, lc fx.Lifecycle, router BaseIpfsRouting, h RawHost, d coredisc.Discovery) error {
	ctx := helpers.LifecycleCtx(mctx, lc)

	// TODO: review: LibP2P doesn't set this as host in config.go, why?
	_ = relay.NewAutoRelay(ctx, h.(*basichost.BasicHost), d, router)
	return nil
}
