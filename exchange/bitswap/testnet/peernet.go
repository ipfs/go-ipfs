package bitswap

import (
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	mockrouting "github.com/ipfs/go-ipfs/routing/mock"
	testutil "github.com/ipfs/go-ipfs/thirdparty/testutil"
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
	mockpeernet "gx/ipfs/QmZ8bCZpMWDbFSh6h2zgTYwrhnjrGM5c9WCzw72SU8p63b/go-libp2p/p2p/net/mock"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

type peernet struct {
	mockpeernet.Mocknet
	routingserver mockrouting.Server
}

func StreamNet(ctx context.Context, net mockpeernet.Mocknet, rs mockrouting.Server) (Network, error) {
	return &peernet{net, rs}, nil
}

func (pn *peernet) Adapter(p testutil.Identity) bsnet.BitSwapNetwork {
	client, err := pn.Mocknet.AddPeer(p.PrivateKey(), p.Address())
	if err != nil {
		panic(err.Error())
	}
	routing := pn.routingserver.ClientWithDatastore(context.TODO(), p, ds.NewMapDatastore())
	return bsnet.NewFromIpfsHost(client, routing)
}

func (pn *peernet) HasPeer(p peer.ID) bool {
	for _, member := range pn.Mocknet.Peers() {
		if p == member {
			return true
		}
	}
	return false
}

var _ Network = &peernet{}
