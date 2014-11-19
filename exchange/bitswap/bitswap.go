// package bitswap implements the IPFS Exchange interface with the BitSwap
// bilateral exchange protocol.
package bitswap

import (
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"

	blocks "github.com/jbenet/go-ipfs/blocks"
	blockstore "github.com/jbenet/go-ipfs/blockstore"
	exchange "github.com/jbenet/go-ipfs/exchange"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	notifications "github.com/jbenet/go-ipfs/exchange/bitswap/notifications"
	strategy "github.com/jbenet/go-ipfs/exchange/bitswap/strategy"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/eventlog"
)

var log = eventlog.Logger("bitswap")

// New initializes a BitSwap instance that communicates over the
// provided BitSwapNetwork. This function registers the returned instance as
// the network delegate.
// Runs until context is cancelled
func New(ctx context.Context, p peer.Peer,
	network bsnet.BitSwapNetwork, routing bsnet.Routing,
	d ds.ThreadSafeDatastore, nice bool) exchange.Interface {

	notif := notifications.New()
	go func() {
		select {
		case <-ctx.Done():
			notif.Shutdown()
		}
	}()

	bs := &bitswap{
		blockstore:    blockstore.NewBlockstore(d),
		notifications: notif,
		strategy:      strategy.New(nice),
		routing:       routing,
		sender:        network,
		wantlist:      u.NewKeySet(),
		blockReq:      make(chan u.Key, 32),
	}
	network.SetDelegate(bs)
	go bs.run(ctx)

	return bs
}

// bitswap instances implement the bitswap protocol.
type bitswap struct {

	// sender delivers messages on behalf of the session
	sender bsnet.BitSwapNetwork

	// blockstore is the local database
	// NB: ensure threadsafety
	blockstore blockstore.Blockstore

	// routing interface for communication
	routing bsnet.Routing

	notifications notifications.PubSub

	blockReq chan u.Key

	// strategy listens to network traffic and makes decisions about how to
	// interact with partners.
	// TODO(brian): save the strategy's state to the datastore
	strategy strategy.Strategy

	wantlist u.KeySet
}

// GetBlock attempts to retrieve a particular block from peers within the
// deadline enforced by the context
//
// TODO ensure only one active request per key
func (bs *bitswap) GetBlock(parent context.Context, k u.Key) (*blocks.Block, error) {

	// make sure to derive a new |ctx| and pass it to children. It's correct to
	// listen on |parent| here, but incorrect to pass |parent| to new async
	// functions. This is difficult to enforce. May this comment keep you safe.

	ctx, cancelFunc := context.WithCancel(parent)
	defer cancelFunc()

	ctx = eventlog.ContextWithMetadata(ctx, eventlog.Uuid("GetBlockRequest"))
	log.Event(ctx, "GetBlockRequestBegin", &k)

	defer func() {
		log.Event(ctx, "GetBlockRequestEnd", &k)
	}()

	bs.wantlist.Add(k)
	promise := bs.notifications.Subscribe(ctx, k)

	select {
	case bs.blockReq <- k:
	case <-parent.Done():
		return nil, parent.Err()
	}

	select {
	case block := <-promise:
		bs.wantlist.Remove(k)
		return &block, nil
	case <-parent.Done():
		return nil, parent.Err()
	}
}

func (bs *bitswap) GetBlocks(parent context.Context, ks []u.Key) (*blocks.Block, error) {
	// TODO: something smart
	return nil, nil
}

func (bs *bitswap) sendWantListTo(ctx context.Context, peers <-chan peer.Peer) error {
	message := bsmsg.New()
	for _, wanted := range bs.wantlist.Keys() {
		message.AddWanted(wanted)
	}
	for peerToQuery := range peers {
		log.Event(ctx, "PeerToQuery", peerToQuery)
		go func(p peer.Peer) {

			log.Event(ctx, "DialPeer", p)
			err := bs.sender.DialPeer(ctx, p)
			if err != nil {
				log.Errorf("Error sender.DialPeer(%s)", p)
				return
			}

			response, err := bs.sender.SendRequest(ctx, p, message)
			if err != nil {
				log.Errorf("Error sender.SendRequest(%s) = %s", p, err)
				return
			}
			// FIXME ensure accounting is handled correctly when
			// communication fails. May require slightly different API to
			// get better guarantees. May need shared sequence numbers.
			bs.strategy.MessageSent(p, message)

			if response == nil {
				return
			}
			bs.ReceiveMessage(ctx, p, response)
		}(peerToQuery)
	}
	return nil
}

func (bs *bitswap) run(ctx context.Context) {
	var sendlist <-chan peer.Peer

	// Every so often, we should resend out our current want list
	rebroadcastTime := time.Second * 5

	// Time to wait before sending out wantlists to better batch up requests
	bufferTime := time.Millisecond * 3
	peersPerSend := 6

	timeout := time.After(rebroadcastTime)
	threshold := 10
	unsent := 0
	for {
		select {
		case <-timeout:
			wantlist := bs.wantlist.Keys()
			if len(wantlist) == 0 {
				continue
			}
			if sendlist == nil {
				// rely on semi randomness of maps
				firstKey := wantlist[0]
				sendlist = bs.routing.FindProvidersAsync(ctx, firstKey, 6)
			}
			err := bs.sendWantListTo(ctx, sendlist)
			if err != nil {
				log.Errorf("error sending wantlist: %s", err)
			}
			sendlist = nil
			timeout = time.After(rebroadcastTime)
		case k := <-bs.blockReq:
			if unsent == 0 {
				sendlist = bs.routing.FindProvidersAsync(ctx, k, peersPerSend)
			}
			unsent++

			if unsent >= threshold {
				// send wantlist to sendlist
				err := bs.sendWantListTo(ctx, sendlist)
				if err != nil {
					log.Errorf("error sending wantlist: %s", err)
				}
				unsent = 0
				timeout = time.After(rebroadcastTime)
				sendlist = nil
			} else {
				// set a timeout to wait for more blocks or send current wantlist

				timeout = time.After(bufferTime)
			}
		case <-ctx.Done():
			return
		}
	}
}

// HasBlock announces the existance of a block to this bitswap service. The
// service will potentially notify its peers.
func (bs *bitswap) HasBlock(ctx context.Context, blk blocks.Block) error {
	log.Debugf("Has Block %v", blk.Key())
	bs.wantlist.Remove(blk.Key())
	bs.sendToPeersThatWant(ctx, blk)
	return bs.routing.Provide(ctx, blk.Key())
}

// TODO(brian): handle errors
func (bs *bitswap) ReceiveMessage(ctx context.Context, p peer.Peer, incoming bsmsg.BitSwapMessage) (
	peer.Peer, bsmsg.BitSwapMessage) {
	log.Debugf("ReceiveMessage from %v", p.Key())
	log.Debugf("Message wantlist: %v", incoming.Wantlist())

	if p == nil {
		log.Error("Received message from nil peer!")
		// TODO propagate the error upward
		return nil, nil
	}
	if incoming == nil {
		log.Error("Got nil bitswap message!")
		// TODO propagate the error upward
		return nil, nil
	}

	// Record message bytes in ledger
	// TODO: this is bad, and could be easily abused.
	// Should only track *useful* messages in ledger
	bs.strategy.MessageReceived(p, incoming) // FIRST

	for _, block := range incoming.Blocks() {
		// TODO verify blocks?
		if err := bs.blockstore.Put(&block); err != nil {
			continue // FIXME(brian): err ignored
		}
		bs.notifications.Publish(block)
		err := bs.HasBlock(ctx, block)
		if err != nil {
			log.Warningf("HasBlock errored: %s", err)
		}
	}

	message := bsmsg.New()
	for _, wanted := range bs.wantlist.Keys() {
		message.AddWanted(wanted)
	}
	for _, key := range incoming.Wantlist() {
		// TODO: might be better to check if we have the block before checking
		//			if we should send it to someone
		if bs.strategy.ShouldSendBlockToPeer(key, p) {
			if block, errBlockNotFound := bs.blockstore.Get(key); errBlockNotFound != nil {
				continue
			} else {
				message.AddBlock(*block)
			}
		}
	}

	bs.strategy.MessageSent(p, message)
	log.Debug("Returning message.")
	return p, message
}

func (bs *bitswap) ReceiveError(err error) {
	log.Errorf("Bitswap ReceiveError: %s", err)
	// TODO log the network error
	// TODO bubble the network error up to the parent context/error logger
}

// send strives to ensure that accounting is always performed when a message is
// sent
func (bs *bitswap) send(ctx context.Context, p peer.Peer, m bsmsg.BitSwapMessage) {
	bs.sender.SendMessage(ctx, p, m)
	bs.strategy.MessageSent(p, m)
}

func (bs *bitswap) sendToPeersThatWant(ctx context.Context, block blocks.Block) {
	log.Debugf("Sending %v to peers that want it", block.Key())

	for _, p := range bs.strategy.Peers() {
		if bs.strategy.BlockIsWantedByPeer(block.Key(), p) {
			log.Debugf("%v wants %v", p, block.Key())
			if bs.strategy.ShouldSendBlockToPeer(block.Key(), p) {
				message := bsmsg.New()
				message.AddBlock(block)
				for _, wanted := range bs.wantlist.Keys() {
					message.AddWanted(wanted)
				}
				bs.send(ctx, p, message)
			}
		}
	}
}
