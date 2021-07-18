package libp2p

import (
	"context"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs/hub"
	libp2p "github.com/libp2p/go-libp2p-core"
	"sort"
	"time"

	"github.com/ipfs/go-ipfs/core/node/helpers"

	"github.com/ipfs/go-ipfs/repo"
	host "github.com/libp2p/go-libp2p-core/host"
	routing "github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	ddht "github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	"github.com/libp2p/go-libp2p-pubsub"
	namesys "github.com/libp2p/go-libp2p-pubsub-router"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"

	"go.uber.org/fx"
)

type BaseIpfsRouting routing.Routing

type Router struct {
	routing.Routing

	Priority int // less = more important
}

type p2pRouterOut struct {
	fx.Out

	Router Router `group:"routers"`
}

type processInitialRoutingIn struct {
	fx.In

	Router routing.Routing `name:"initialrouting"`

	// For setting up experimental DHT client
	Host      host.Host
	Repo      repo.Repo
	Validator record.Validator
}

type processInitialRoutingOut struct {
	fx.Out

	Router    Router `group:"routers"`
	DHT       *ddht.DHT
	DHTClient routing.Routing `name:"dhtc"`
	BaseRT    BaseIpfsRouting
}

func BaseRouting(experimentalDHTClient bool) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, in processInitialRoutingIn) (out processInitialRoutingOut, err error) {
		var dr *ddht.DHT
		if dht, ok := in.Router.(*ddht.DHT); ok {
			dr = dht

			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return dr.Close()
				},
			})
		}

		if dr != nil && experimentalDHTClient {
			cfg, err := in.Repo.Config()
			if err != nil {
				return out, err
			}
			bspeers, err := cfg.BootstrapPeers()
			if err != nil {
				return out, err
			}

			expClient, err := fullrt.NewFullRT(in.Host,
				dht.DefaultPrefix,
				fullrt.DHTOption(
					dht.Validator(in.Validator),
					dht.Datastore(in.Repo.Datastore()),
					dht.BootstrapPeers(bspeers...),
					dht.BucketSize(20),
				),
			)
			if err != nil {
				return out, err
			}

			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return expClient.Close()
				},
			})

			return processInitialRoutingOut{
				Router: Router{
					Routing:  expClient,
					Priority: 1000,
				},
				DHT:       dr,
				DHTClient: expClient,
				BaseRT:    expClient,
			}, nil
		}

		return processInitialRoutingOut{
			Router: Router{
				Priority: 1000,
				Routing:  in.Router,
			},
			DHT:       dr,
			DHTClient: dr,
			BaseRT:    in.Router,
		}, nil
	}
}

type p2pOnlineRoutingIn struct {
	fx.In

	Routers   []Router `group:"routers"`
	Validator record.Validator
}

func Routing(in p2pOnlineRoutingIn) routing.Routing {
	routers := in.Routers

	sort.SliceStable(routers, func(i, j int) bool {
		return routers[i].Priority < routers[j].Priority
	})

	irouters := make([]routing.Routing, 0, len(routers))
	for _, v := range routers {
		if v.Routing != nil {
			irouters = append(irouters, v.Routing)
		}
	}

	return routinghelpers.Tiered{
		Routers:   irouters,
		Validator: in.Validator,
	}
}

type p2pPSRoutingIn struct {
	fx.In

	BaseIpfsRouting BaseIpfsRouting
	Validator       record.Validator
	Host            host.Host
	PubSub          *pubsub.PubSub `optional:"true"`
}

func PubsubRouter(mctx helpers.MetricsCtx, lc fx.Lifecycle, in p2pPSRoutingIn) (p2pRouterOut, *namesys.PubsubValueStore, error) {
	psRouter, err := namesys.NewPubsubValueStore(
		helpers.LifecycleCtx(mctx, lc),
		in.Host,
		in.PubSub,
		in.Validator,
		namesys.WithRebroadcastInterval(time.Minute),
	)

	if err != nil {
		return p2pRouterOut{}, nil, err
	}

	return p2pRouterOut{
		Router: Router{
			Routing: &routinghelpers.Compose{
				ValueStore: &routinghelpers.LimitedValueStore{
					ValueStore: psRouter,
					Namespaces: []string{"ipns"},
				},
			},
			Priority: 100,
		},
	}, psRouter, nil
}

func Hub(clientEnabled, serverEnabled bool) interface{} {
	const topicID = "/ipfs/content-routing/1.0.0"
	return func(lc fx.Lifecycle, in p2pPSRoutingIn, bs blockstore.GCBlockstore) (out p2pRouterOut, client *hub.Client, server *hub.Server, err error) {
		var topic *pubsub.Topic
		topic, err = in.PubSub.Join(topicID)
		if err != nil {
			return p2pRouterOut{}, nil, nil, err
		}

		if clientEnabled {
			out, client, err = HubClient(topic)
			if err != nil {
				return p2pRouterOut{}, nil, nil, err
			}
		}

		if serverEnabled {
			server, err = HubServer(lc, topic, bs, in.Host)
			if err != nil {
				return p2pRouterOut{}, nil, nil, err
			}
		}

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return nil
				//return topic.Close()
			},
		})

		return out, client, server, nil
	}
}

func HubServer(lc fx.Lifecycle, topic *pubsub.Topic, bs blockstore.GCBlockstore, h libp2p.Host) (*hub.Server, error) {
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	server, err := hub.NewServer(sub, bs, h)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return server.Start()
		},
		OnStop: func(ctx context.Context) error {
			return server.Close()
		},
	})

	return server, nil
}

func HubClient(topic *pubsub.Topic) (p2pRouterOut, *hub.Client, error) {
	client, err := hub.NewClient(topic)
	if err != nil {
		return p2pRouterOut{}, nil, err
	}

	return p2pRouterOut{
		Router: Router{
			Routing: &routinghelpers.Compose{
				ContentRouting: client,
			},
			Priority: 500,
		},
	}, client, nil
}
