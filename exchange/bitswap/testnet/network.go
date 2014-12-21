package bitswap

import (
	"errors"
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	peer "github.com/jbenet/go-ipfs/peer"
	delay "github.com/jbenet/go-ipfs/util/delay"
)

type Network interface {
	Adapter(peer.ID) bsnet.BitSwapNetwork

	HasPeer(peer.ID) bool

	SendMessage(
		ctx context.Context,
		from peer.ID,
		to peer.ID,
		message bsmsg.BitSwapMessage) error

	SendRequest(
		ctx context.Context,
		from peer.ID,
		to peer.ID,
		message bsmsg.BitSwapMessage) (
		incoming bsmsg.BitSwapMessage, err error)
}

// network impl

func VirtualNetwork(d delay.D) Network {
	return &network{
		clients: make(map[peer.ID]bsnet.Receiver),
		delay:   d,
	}
}

type network struct {
	clients map[peer.ID]bsnet.Receiver
	delay   delay.D
}

func (n *network) Adapter(p peer.ID) bsnet.BitSwapNetwork {
	client := &networkClient{
		local:   p,
		network: n,
	}
	n.clients[p] = client
	return client
}

func (n *network) HasPeer(p peer.ID) bool {
	_, found := n.clients[p]
	return found
}

// TODO should this be completely asynchronous?
// TODO what does the network layer do with errors received from services?
func (n *network) SendMessage(
	ctx context.Context,
	from peer.ID,
	to peer.ID,
	message bsmsg.BitSwapMessage) error {

	receiver, ok := n.clients[to]
	if !ok {
		return errors.New("Cannot locate peer on network")
	}

	// nb: terminate the context since the context wouldn't actually be passed
	// over the network in a real scenario

	go n.deliver(receiver, from, message)

	return nil
}

func (n *network) deliver(
	r bsnet.Receiver, from peer.ID, message bsmsg.BitSwapMessage) error {
	if message == nil || from == "" {
		return errors.New("Invalid input")
	}

	n.delay.Wait()

	nextPeer, nextMsg := r.ReceiveMessage(context.TODO(), from, message)

	if (nextPeer == "" && nextMsg != nil) || (nextMsg == nil && nextPeer != "") {
		return errors.New("Malformed client request")
	}

	if nextPeer == "" && nextMsg == nil { // no response to send
		return nil
	}

	nextReceiver, ok := n.clients[nextPeer]
	if !ok {
		return errors.New("Cannot locate peer on network")
	}
	go n.deliver(nextReceiver, nextPeer, nextMsg)
	return nil
}

// TODO
func (n *network) SendRequest(
	ctx context.Context,
	from peer.ID,
	to peer.ID,
	message bsmsg.BitSwapMessage) (
	incoming bsmsg.BitSwapMessage, err error) {

	r, ok := n.clients[to]
	if !ok {
		return nil, errors.New("Cannot locate peer on network")
	}
	nextPeer, nextMsg := r.ReceiveMessage(context.TODO(), from, message)

	// TODO dedupe code
	if (nextPeer == "" && nextMsg != nil) || (nextMsg == nil && nextPeer != "") {
		r.ReceiveError(errors.New("Malformed client request"))
		return nil, nil
	}

	// TODO dedupe code
	if nextPeer == "" && nextMsg == nil {
		return nil, nil
	}

	// TODO test when receiver doesn't immediately respond to the initiator of the request
	if nextPeer == from {
		go func() {
			nextReceiver, ok := n.clients[nextPeer]
			if !ok {
				// TODO log the error?
			}
			n.deliver(nextReceiver, nextPeer, nextMsg)
		}()
		return nil, nil
	}
	return nextMsg, nil
}

type networkClient struct {
	local peer.ID
	bsnet.Receiver
	network Network
}

func (nc *networkClient) SendMessage(
	ctx context.Context,
	to peer.ID,
	message bsmsg.BitSwapMessage) error {
	return nc.network.SendMessage(ctx, nc.local, to, message)
}

func (nc *networkClient) SendRequest(
	ctx context.Context,
	to peer.ID,
	message bsmsg.BitSwapMessage) (incoming bsmsg.BitSwapMessage, err error) {
	return nc.network.SendRequest(ctx, nc.local, to, message)
}

func (nc *networkClient) DialPeer(ctx context.Context, p peer.ID) error {
	// no need to do anything because dialing isn't a thing in this test net.
	if !nc.network.HasPeer(p) {
		return fmt.Errorf("Peer not in network: %s", p)
	}
	return nil
}

func (nc *networkClient) SetDelegate(r bsnet.Receiver) {
	nc.Receiver = r
}
