/*
Package coreapi provides direct access to the core commands in IPFS. If you are
embedding IPFS directly in your Go program, this package is the public
interface you should use to read and write files or otherwise control IPFS.

If you are running IPFS as a separate process, you should use `go-ipfs-api` to
work with it via HTTP. As we finalize the interfaces here, `go-ipfs-api` will
transparently adopt them so you can use the same code with either package.

**NOTE: this package is experimental.** `go-ipfs` has mainly been developed
as a standalone application and library-style use of this package is still new.
Interfaces here aren't yet completely stable.
*/
package coreapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/node"
	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/provider"
	"github.com/ipfs/go-ipfs/repo"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-interface"
	offlinexch "github.com/ipfs/go-ipfs-exchange-offline"
	offlineroute "github.com/ipfs/go-ipfs-routing/offline"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	dag "github.com/ipfs/go-merkledag"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	p2phost "github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	routing "github.com/libp2p/go-libp2p-core/routing"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	record "github.com/libp2p/go-libp2p-record"
)

var log = logging.Logger("core/coreapi")

type CoreAPI struct {
	nctx context.Context

	identity   peer.ID
	privateKey ci.PrivKey

	repo       repo.Repo
	blockstore blockstore.GCBlockstore
	baseBlocks blockstore.Blockstore
	pinning    pin.Pinner

	blocks bserv.BlockService
	dag    ipld.DAGService

	peerstore       pstore.Peerstore
	peerHost        p2phost.Host
	recordValidator record.Validator
	exchange        exchange.Interface

	namesys namesys.NameSystem
	routing routing.Routing

	provider provider.System

	pubSub *pubsub.PubSub

	checkPublishAllowed func() error
	checkOnline         func(allowOffline bool) error

	// ONLY for re-applying options in WithOptions, DO NOT USE ANYWHERE ELSE
	nd         *core.IpfsNode
	parentOpts options.ApiSettings
}

// NewCoreAPI creates new instance of IPFS CoreAPI backed by go-ipfs Node.
func NewCoreAPI(n *core.IpfsNode, opts ...options.ApiOption) (*CoreAPI, error) {
	parentOpts, err := options.ApiOptions()
	if err != nil {
		return nil, err
	}

	return (&CoreAPI{nd: n, parentOpts: *parentOpts}).withOptions(opts...)
}

// Unixfs returns the UnixfsAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Unixfs() coreiface.UnixfsAPI {
	return (*UnixfsAPI)(api)
}

// Block returns the BlockAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Block() coreiface.BlockAPI {
	return (*BlockAPI)(api)
}

// Dag returns the DagAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Dag() coreiface.APIDagService {
	return &dagAPI{
		api.dag,
		api,
	}
}

// Name returns the NameAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Name() coreiface.NameAPI {
	return (*NameAPI)(api)
}

// Key returns the KeyAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Key() coreiface.KeyAPI {
	return (*KeyAPI)(api)
}

// Object returns the ObjectAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Object() coreiface.ObjectAPI {
	return (*ObjectAPI)(api)
}

// Pin returns the PinAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Pin() coreiface.PinAPI {
	return (*PinAPI)(api)
}

// Dht returns the DhtAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Dht() coreiface.DhtAPI {
	return (*DhtAPI)(api)
}

// Swarm returns the SwarmAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) Swarm() coreiface.SwarmAPI {
	return (*SwarmAPI)(api)
}

// PubSub returns the PubSubAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) PubSub() coreiface.PubSubAPI {
	return (*PubSubAPI)(api)
}

// WithOptions returns api with global options applied
func (api *CoreAPI) WithOptions(opts ...options.ApiOption) (coreiface.CoreAPI, error) {
	return api.withOptions(opts...)
}

func (api *CoreAPI) withOptions(opts ...options.ApiOption) (*CoreAPI, error) {
	settings := api.parentOpts // make sure to copy
	_, err := options.ApiOptionsTo(&settings, opts...)
	if err != nil {
		return nil, err
	}

	if api.nd == nil {
		return nil, errors.New("cannot apply options to api without node")
	}

	n := api.nd

	subApi := &CoreAPI{
		nctx: n.Context(),

		identity:   n.Identity,
		privateKey: n.PrivateKey,

		repo:       n.Repo,
		blockstore: n.Blockstore,
		baseBlocks: n.BaseBlocks,
		pinning:    n.Pinning,

		blocks: n.Blocks,
		dag:    n.DAG,

		peerstore:       n.Peerstore,
		peerHost:        n.PeerHost,
		namesys:         n.Namesys,
		recordValidator: n.RecordValidator,
		exchange:        n.Exchange,
		routing:         n.Routing,

		provider: n.Provider,

		pubSub: n.PubSub,

		nd:         n,
		parentOpts: settings,
	}

	subApi.checkOnline = func(allowOffline bool) error {
		if !n.IsOnline && !allowOffline {
			return coreiface.ErrOffline
		}
		return nil
	}

	subApi.checkPublishAllowed = func() error {
		if n.Mounts.Ipns != nil && n.Mounts.Ipns.IsActive() {
			return errors.New("cannot manually publish while IPNS is mounted")
		}
		return nil
	}

	if settings.Offline {
		cfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}

		cs := cfg.Ipns.ResolveCacheSize
		if cs == 0 {
			cs = node.DefaultIpnsCacheSize
		}
		if cs < 0 {
			return nil, fmt.Errorf("cannot specify negative resolve cache size")
		}

		subApi.routing = offlineroute.NewOfflineRouter(subApi.repo.Datastore(), subApi.recordValidator)
		subApi.namesys = namesys.NewNameSystem(subApi.routing, subApi.repo.Datastore(), cs)
		subApi.provider = provider.NewOfflineProvider()

		subApi.peerstore = nil
		subApi.peerHost = nil
		subApi.recordValidator = nil
	}

	if settings.Offline || !settings.FetchBlocks {
		subApi.exchange = offlinexch.Exchange(subApi.blockstore)
		subApi.blocks = bserv.New(subApi.blockstore, subApi.exchange)
		subApi.dag = dag.NewDAGService(subApi.blocks)
	}

	return subApi, nil
}

// Node returns the underlying ipfs node object. Don't use unless absolutely
// necessary. Options applied to CoreAPI won't apply to node returned form this
// method.
//
// If you need to use this method because some functionality is missing from the
// CoreAPI interfaces, please file an issue in https://github.com/ipfs/go-ipfs/
//
// Deprecated: This method will eventually be removed
func (api *CoreAPI) Node() *core.IpfsNode {
	return api.nd
}

// getSession returns new api backed by the same node with a read-only session DAG
func (api *CoreAPI) getSession(ctx context.Context) *CoreAPI {
	sesApi := *api

	// TODO: We could also apply this to api.blocks, and compose into writable api,
	// but this requires some changes in blockservice/merkledag
	sesApi.dag = dag.NewReadOnlyDagService(dag.NewSession(ctx, api.dag))

	return &sesApi
}
