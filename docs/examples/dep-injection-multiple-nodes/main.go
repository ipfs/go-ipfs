package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	mathrand "math/rand"
	"sync"
	"time"

	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-datastore"

	blockstore "github.com/ipfs/go-ipfs-blockstore"
	config "github.com/ipfs/go-ipfs-config"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/bootstrap"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node"
	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/p2p"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/jbenet/goprocess"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/prometheus/common/log"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"

	"github.com/ipfs/go-metrics-interface"
	"go.uber.org/fx"
)

var randReader *mathrand.Rand

// RandReader generates a random stream of bytes
func RandReader(len int) io.Reader {
	if randReader == nil {
		randReader = mathrand.New(mathrand.NewSource(2))
	}
	data := make([]byte, len)
	randReader.Read(data)
	return bytes.NewReader(data)
}

// baseProcess creates a goprocess which is closed when the lifecycle signals it to stop
func baseProcess(lc fx.Lifecycle) goprocess.Process {
	p := goprocess.WithParent(goprocess.Background())
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return p.Close()
		},
	})
	return p
}

// setConfig manually injects dependencies for the IPFS nodes.
func setConfig(ctx context.Context) fx.Option {

	// Create new Datastore
	d := ds.NewMapDatastore()
	// Initialize config.
	cfg := &config.Config{}
	// Generate new KeyPair instead of using existing one.
	priv, pub, err := ci.GenerateKeyPairWithReader(ci.RSA, 2048, rand.Reader)
	if err != nil {
		panic(err)
	}
	// Generate PeerID
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		panic(err)
	}
	// Get PrivKey
	privkeyb, err := priv.Bytes()
	if err != nil {
		panic(err)
	}
	// Use defaultBootstrap
	cfg.Bootstrap = config.DefaultBootstrapAddresses

	//Allow the node to start in any available port. We do not use default ones.
	cfg.Addresses.Swarm = []string{
		"/ip4/0.0.0.0/tcp/0",
		"/ip6/::/tcp/0",
		"/ip4/0.0.0.0/udp/0/quic",
		"/ip6/::/udp/0/quic",
	}
	cfg.Identity.PeerID = pid.Pretty()
	cfg.Identity.PrivKey = base64.StdEncoding.EncodeToString(privkeyb)

	// Repo structure that encapsulate the config and datastore for dependency injection.
	buildRepo := &repo.Mock{
		D: dsync.MutexWrap(d),
		C: *cfg,
	}
	repoOption := fx.Provide(func(lc fx.Lifecycle) repo.Repo {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return buildRepo.Close()
			},
		})
		return buildRepo
	})

	// Enable metrics in the node.
	metricsCtx := fx.Provide(func() helpers.MetricsCtx {
		return helpers.MetricsCtx(ctx)
	})

	// Use DefaultHostOptions
	hostOption := fx.Provide(func() libp2p.HostOption {
		return libp2p.DefaultHostOption
	})

	// Use libp2p.DHTOption. Could also use DHTClientOption.
	routingOption := fx.Provide(func() libp2p.RoutingOption {
		// return libp2p.DHTClientOption
		return libp2p.DHTOption
	})

	// Uncomment if you want to set Graphsync as exchange interface.
	// gsExchange := func(mctx helpers.MetricsCtx, lc fx.Lifecycle,
	// 	host host.Host, rt routing.Routing, bs blockstore.GCBlockstore) exchange.Interface {

	// 	// TODO: Graphsync currently doesn't follow exchange.Interface. Is missing Close()
	// 	ctx := helpers.LifecycleCtx(mctx, lc)
	// 	network := network.NewFromLibp2pHost(host)
	// 	ipldBridge := ipldbridge.NewIPLDBridge()
	// 	gsExch := gsimpl.New(ctx,
	// 		network, ipldBridge,
	// 		storeutil.LoaderForBlockstore(bs),
	// 		storeutil.StorerForBlockstore(bs),
	// 	)

	// 	lc.Append(fx.Hook{
	// 		OnStop: func(ctx context.Context) error {
	// 			if exch.Close != nil {
	// 				return exch.Close()
	// 			}
	// 			return nil

	// 		},
	// 	})
	// 	return exch
	// }

	// Initializing bitswap exchange
	bsExchange := func(mctx helpers.MetricsCtx, lc fx.Lifecycle,
		host host.Host, rt routing.Routing, bs blockstore.GCBlockstore) exchange.Interface {
		bitswapNetwork := network.NewFromIpfsHost(host, rt)
		exch := bitswap.New(helpers.LifecycleCtx(mctx, lc), bitswapNetwork, bs)

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return exch.Close()
			},
		})
		return exch
	}

	// Return repo datastore
	repoDS := func(repo repo.Repo) datastore.Datastore {
		return d
	}

	// Assign some defualt values.
	var repubPeriod, recordLifetime time.Duration
	ipnsCacheSize := cfg.Ipns.ResolveCacheSize
	enableRelay := cfg.Swarm.Transports.Network.Relay.WithDefault(!cfg.Swarm.DisableRelay) //nolint

	// Inject all dependencies for the node.
	// Many of the default dependencies being used. If you want to manually set any of them
	// follow: https://github.com/ipfs/go-ipfs/blob/master/core/node/groups.go
	return fx.Options(
		// RepoConfigurations
		repoOption,
		hostOption,
		routingOption,
		metricsCtx,

		// Setting baseProcess
		fx.Provide(baseProcess),

		// Storage configuration
		fx.Provide(repoDS),
		fx.Provide(node.BaseBlockstoreCtor(blockstore.DefaultCacheOpts(),
			false, cfg.Datastore.HashOnRead)),
		fx.Provide(node.GcBlockstoreCtor),

		// Identity dependencies
		node.Identity(cfg),

		//IPNS dependencies
		node.IPNS,

		// Network dependencies
		// fx.Provide(gsExchange),  // Uncomment to set graphsync exchange
		fx.Provide(bsExchange),
		fx.Provide(node.Namesys(ipnsCacheSize)),
		fx.Provide(node.Peering),
		node.PeerWith(cfg.Peering.Peers...),

		fx.Invoke(node.IpnsRepublisher(repubPeriod, recordLifetime)),

		fx.Provide(p2p.New),

		// Libp2p dependencies
		node.BaseLibP2P,
		fx.Provide(libp2p.AddrFilters(cfg.Swarm.AddrFilters)),
		fx.Provide(libp2p.AddrsFactory(cfg.Addresses.Announce, cfg.Addresses.NoAnnounce)),
		fx.Provide(libp2p.SmuxTransport(cfg.Swarm.Transports)),
		fx.Provide(libp2p.Relay(enableRelay, cfg.Swarm.EnableRelayHop)),
		fx.Provide(libp2p.Transports(cfg.Swarm.Transports)),
		fx.Invoke(libp2p.StartListening(cfg.Addresses.Swarm)),
		fx.Invoke(libp2p.SetupDiscovery(cfg.Discovery.MDNS.Enabled, cfg.Discovery.MDNS.Interval)),
		fx.Provide(libp2p.Routing),
		fx.Provide(libp2p.BaseRouting),

		// Here you can see some more of the libp2p dependencies you could set.
		// fx.Provide(libp2p.Security(!bcfg.DisableEncryptedConnections, cfg.Swarm.Transports)),
		// maybeProvide(libp2p.PubsubRouter, bcfg.getOpt("ipnsps")),
		// maybeProvide(libp2p.BandwidthCounter, !cfg.Swarm.DisableBandwidthMetrics),
		// maybeProvide(libp2p.NatPortMap, !cfg.Swarm.DisableNatPortMap),
		// maybeProvide(libp2p.AutoRelay, cfg.Swarm.EnableAutoRelay),
		// autonat,		// Sets autonat
		// connmgr,		// Set connection manager
		// ps,			// Sets pubsub router
		// disc,		// Sets discovery service
		node.OnlineProviders(cfg.Experimental.StrategicProviding, cfg.Reprovider.Strategy, cfg.Reprovider.Interval),

		// Core configuration
		node.Core,
	)
}

// NewNode constructs and returns an IpfsNode using the given cfg.
func NewNode(ctx context.Context) (*core.IpfsNode, func() error, error) {
	// save this context as the "lifetime" ctx.
	lctx := ctx

	// derive a new context that ignores cancellations from the lifetime ctx.
	ctx, cancel := context.WithCancel(ctx)

	// add a metrics scope.
	ctx = metrics.CtxScope(ctx, "ipfs")

	n := &core.IpfsNode{}

	app := fx.New(
		// Inject dependencies in the node.
		setConfig(ctx),

		fx.NopLogger,
		fx.Extract(n),
	)

	var once sync.Once
	var stopErr error
	stopNode := func() error {
		once.Do(func() {
			stopErr = app.Stop(context.Background())
			if stopErr != nil {
				log.Error("failure on stop: ", stopErr)
			}
			// Cancel the context _after_ the app has stopped.
			cancel()
		})
		return stopErr
	}
	// Set node to Online mode.
	n.IsOnline = true

	go func() {
		// Shut down the application if the lifetime context is canceled.
		// NOTE: we _should_ stop the application by calling `Close()`
		// on the process. But we currently manage everything with contexts.
		select {
		case <-lctx.Done():
			err := stopNode()
			if err != nil {
				log.Error("failure on stop: ", err)
			}
		case <-ctx.Done():
		}
	}()

	if app.Err() != nil {
		return nil, nil, app.Err()
	}

	if err := app.Start(ctx); err != nil {
		return nil, nil, err
	}

	return n, stopNode, n.Bootstrap(bootstrap.DefaultBootstrapConfig)
}

func main() {

	// FileSize to be generated randomly for the execution.
	FileSize := 1324643

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Spawn two different nodes with the same configuration.
	// Each will be started in a different port.
	n1, n1Close, err := NewNode(ctx)
	fmt.Println("[*] Spawned first node listening at: ", n1.PeerHost.Addrs())
	if err != nil {
		panic(err)
	}
	n2, n2Close, err := NewNode(ctx)
	fmt.Println("[*] Spawned first node listening at: ", n2.PeerHost.Addrs())
	if err != nil {
		panic(err)
	}

	// Configuring core APIs
	api1, err := coreapi.NewCoreAPI(n1)
	if err != nil {
		panic(err)
	}
	api2, err := coreapi.NewCoreAPI(n2)

	// Connecting from n1 to n2
	err = api1.Swarm().Connect(ctx, *host.InfoFromHost(n2.PeerHost))
	if err != nil {
		fmt.Printf("failed to connect: %s\n", err)
	}
	fmt.Println("[*] Connected fron node1 to node2")

	// Randomly generate the file.
	randomFile := files.NewReaderFile(RandReader(FileSize))
	// Add the file to the network.
	cidRandom, err := api1.Unixfs().Add(ctx, randomFile)
	if err != nil {
		panic(fmt.Errorf("Could not add random: %s", err))
	}
	fmt.Println("[*] Added a test file to the network:", cidRandom)

	// Retrieve the DAG structure from the other node.
	fmt.Printf("[*] Searching for %v from node 2\n", cidRandom)
	f, err := api2.Unixfs().Get(ctx, cidRandom)
	if err != nil {
		panic(fmt.Errorf("Could find file in IPFS: %s", err))
	}
	// Traverse the full graph and write the file in /tmp/
	// If we don't write the file we only get the DagReader in f.
	err = files.WriteTo(f, "/tmp/"+time.Now().String())
	if err != nil {
		panic(fmt.Errorf("Could not write retrieved file: %s", err))
	}
	// Size of the file.
	s, _ := f.Size()
	fmt.Println("[*] Retrieved file with size: ", s)
	// Close both nodes.
	n1Close()
	fmt.Println("[*] Gracefully closed node 1")
	n2Close()
	fmt.Println("[*] Gracefully closed node 2")
}
