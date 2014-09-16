package transmission

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	bsmsg "github.com/jbenet/go-ipfs/bitswap/message"
	peer "github.com/jbenet/go-ipfs/peer"
)

type Sender interface {
	SendMessage(ctx context.Context, destination *peer.Peer, message bsmsg.Exportable) error
	SendRequest(ctx context.Context, destination *peer.Peer, outgoing bsmsg.Exportable) (
		incoming bsmsg.BitSwapMessage, err error)
}

// TODO(brian): consider returning a NetMessage
type Receiver interface {
	ReceiveMessage(
		ctx context.Context, sender *peer.Peer, incoming bsmsg.BitSwapMessage) (
		outgoing bsmsg.BitSwapMessage, destination *peer.Peer, err error)
}

// TODO(brian): move this to go-ipfs/net package
type NetworkService interface {
	SendRequest(ctx context.Context, m netmsg.NetMessage) (netmsg.NetMessage, error)
	SendMessage(ctx context.Context, m netmsg.NetMessage) error
	SetHandler(netservice.Handler)
}
