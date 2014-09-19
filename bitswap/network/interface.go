package network

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	netservice "github.com/jbenet/go-ipfs/net/service"

	bsmsg "github.com/jbenet/go-ipfs/bitswap/message"
	netmsg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
)

// NetAdapter mediates the exchange's communication with the network.
type NetAdapter interface {

	// SendMessage sends a BitSwap message to a peer.
	SendMessage(
		context.Context,
		*peer.Peer,
		bsmsg.BitSwapMessage) error

	// SendRequest sends a BitSwap message to a peer and waits for a response.
	SendRequest(
		context.Context,
		*peer.Peer,
		bsmsg.BitSwapMessage) (incoming bsmsg.BitSwapMessage, err error)

	// SetDelegate registers the Reciver to handle messages received from the
	// network.
	SetDelegate(Receiver)
}

//Receiver gets the bitswap message from the sender and outputs the destination for it
type Receiver interface {
	ReceiveMessage(
		ctx context.Context, sender *peer.Peer, incoming bsmsg.BitSwapMessage) (
		destination *peer.Peer, outgoing bsmsg.BitSwapMessage, err error)
}

// TODO(brian): move this to go-ipfs/net package
type NetService interface {
	SendRequest(ctx context.Context, m netmsg.NetMessage) (netmsg.NetMessage, error)
	SendMessage(ctx context.Context, m netmsg.NetMessage) error
	SetHandler(netservice.Handler)
}
