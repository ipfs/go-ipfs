package node

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	bsutil "github.com/ipfs/go-bitswap/util"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-filestore"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-interface"
	"github.com/ipfs/go-ipfs-pinner"
	"github.com/ipfs/go-ipfs-pinner/dspinner"
	"github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-mfs"
	"github.com/ipfs/go-unixfs"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/repo"
)

var bsLog = logging.Logger("bitswap")

// BlockService creates new blockservice which provides an interface to fetch content-addressable blocks
func BlockService(lc fx.Lifecycle, bs blockstore.Blockstore, rem exchange.Interface) blockservice.BlockService {
	bsvc := blockservice.New(bs, rem)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return bsvc.Close()
		},
	})

	return bsvc
}

// Pinning creates new pinner which tells GC which blocks should be kept
func Pinning(bstore blockstore.Blockstore, ds format.DAGService, repo repo.Repo) (pin.Pinner, error) {
	rootDS := repo.Datastore()

	syncFn := func() error {
		if err := rootDS.Sync(blockstore.BlockPrefix); err != nil {
			return err
		}
		return rootDS.Sync(filestore.FilestorePrefix)
	}
	syncDs := &syncDagService{ds, syncFn}

	ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Minute)
	defer cancel()

	pinning, err := dspinner.New(ctx, rootDS, syncDs)
	if err != nil {
		return nil, err
	}

	return pinning, nil
}

var (
	_ merkledag.SessionMaker = new(syncDagService)
	_ format.DAGService      = new(syncDagService)
)

// syncDagService is used by the Pinner to ensure data gets persisted to the underlying datastore
type syncDagService struct {
	format.DAGService
	syncFn func() error
}

func (s *syncDagService) Sync() error {
	return s.syncFn()
}

func (s *syncDagService) Session(ctx context.Context) format.NodeGetter {
	return merkledag.NewSession(ctx, s.DAGService)
}

// Dag creates new DAGService
func Dag(bs blockservice.BlockService) format.DAGService {
	return merkledag.NewDAGService(bs)
}

// OnlineExchange creates new LibP2P backed block exchange (BitSwap)
func OnlineExchange(provide bool) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, host host.Host, rt routing.Routing, bs blockstore.GCBlockstore) exchange.Interface {
		bitswapNetwork := network.NewFromIpfsHost(host, rt)
		exch := bitswap.New(helpers.LifecycleCtx(mctx, lc), bitswapNetwork, bs, bitswap.ProvideEnabled(provide))
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return exch.Close()
			},
		})
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				// Monitoring configuration settings as environment variables
				// with the `BS_CFG_` prefix.
				// * `BS_CFG_TIMEOUT`: time interval between test (CID requests).
				//  Also the time we wait for *all* CID requests to be responded
				//  before reporting an error.
				// * `BS_CFG_CID_REQ_NUM`: number of CIDs requested in total for
				//   the group of valid and existing CIDs (that will have a BS
				//  `BLOCK` response) and the group of invalid (shortened)
				//  nonexistent CIDs (that will have a `DONT_HAVE` response).
				//  They are read and parsed every test run to be modified without
				//  restarting the node. (The timeout is usually in the order of
				//  seconds and this is not a measurable performance penalty.) They
				//  need to be set *always* otherwise we report an error (they
				//  are crucial for the test to be meaningful).
				// FIXME: This should actually be part of the config file
				//  to use the node API to change them on the fly but I'd
				//  like to avoid modifying yet another dependency for now.
				go func() {
					for {
						timeout, successTimeout := parseNonZeroIntConfig("BS_CFG_TIMEOUT")
						cidNum, successCid := parseNonZeroIntConfig("BS_CFG_CID_REQ_NUM")
						if successTimeout == false || successCid == false {
							time.Sleep(time.Second * 5)
							continue
						}

						// We do the check every `timeout` seconds. This time is also
						// as much as we are willing to wait for the response
						// on all CIDs requested. This means we only do *one*
						// test at a time.
						testContext, _ := context.WithTimeout(ctx, time.Second*time.Duration(timeout))

						checkBitswapResponse(testContext, host.ID(), cidNum)

						select {
						case <-helpers.LifecycleCtx(mctx, lc).Done():
							return
						case <-testContext.Done():
						}
					}
				}()
				return nil
			},
		})
		return exch

	}
}

func parseNonZeroIntConfig(configString string) (int, bool) {
	configValueString := os.Getenv(configString)
	if configValueString == "" {
		bsLog.Errorf("%s not set", configString)
		return 0, false
	}
	configInt, err := strconv.Atoi(configValueString)
	if err != nil {
		bsLog.Errorf("error parsing %s: %s",
			configString, err)
		return 0, false
	}
	if configInt == 0 {
		bsLog.Errorf("%s set to zero seconds", configString)
		return 0, false
	}
	return configInt, true
}

func checkBitswapResponse(ctx context.Context,
	localPeer peer.ID,
	cidNum int,
) {
	bsLog.Debug("checking BitSwap response") // FIXME: Add more info here. Connection type?
	missingCids, err := bsutil.CheckBitswapCID(ctx, localPeer, cidNum)
	if err != nil {
		if err != context.Canceled {
			bsLog.Warnf("error in CheckBitswapCID: %s", err)
		}
	} else if len(missingCids) > 0 {
		// Note this is an error, not a warning. This is the
		// true error case we are monitoring for (the rest
		// is noise and probably errors in this tool).
		bsLog.Errorf("CheckBitswapCID: did not get HAVE/DONT-HAVE response on CIDs: %v", missingCids)
		// FIXME: Log also the timeout we waited for the response.
	}
}

// Files loads persisted MFS root
func Files(mctx helpers.MetricsCtx, lc fx.Lifecycle, repo repo.Repo, dag format.DAGService) (*mfs.Root, error) {
	dsk := datastore.NewKey("/local/filesroot")
	pf := func(ctx context.Context, c cid.Cid) error {
		rootDS := repo.Datastore()
		if err := rootDS.Sync(blockstore.BlockPrefix); err != nil {
			return err
		}
		if err := rootDS.Sync(filestore.FilestorePrefix); err != nil {
			return err
		}

		if err := rootDS.Put(dsk, c.Bytes()); err != nil {
			return err
		}
		return rootDS.Sync(dsk)
	}

	var nd *merkledag.ProtoNode
	val, err := repo.Datastore().Get(dsk)
	ctx := helpers.LifecycleCtx(mctx, lc)

	switch {
	case err == datastore.ErrNotFound || val == nil:
		nd = unixfs.EmptyDirNode()
		err := dag.Add(ctx, nd)
		if err != nil {
			return nil, fmt.Errorf("failure writing to dagstore: %s", err)
		}
	case err == nil:
		c, err := cid.Cast(val)
		if err != nil {
			return nil, err
		}

		rnd, err := dag.Get(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("error loading filesroot from DAG: %s", err)
		}

		pbnd, ok := rnd.(*merkledag.ProtoNode)
		if !ok {
			return nil, merkledag.ErrNotProtobuf
		}

		nd = pbnd
	default:
		return nil, err
	}

	root, err := mfs.NewRoot(ctx, dag, nd, pf)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return root.Close()
		},
	})

	return root, err
}
