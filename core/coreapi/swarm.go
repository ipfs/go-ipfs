package coreapi

import (
	"context"
	"sort"
	"time"

	coreiface "github.com/ipfs/interface-go-ipfs-core"
	inet "github.com/libp2p/go-libp2p-core/network"
	peer "github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	protocol "github.com/libp2p/go-libp2p-core/protocol"
	swarm "github.com/libp2p/go-libp2p-swarm"
	ma "github.com/multiformats/go-multiaddr"
)

type SwarmAPI CoreAPI

type connInfo struct {
	peerstore pstore.Peerstore
	conn      inet.Conn
	dir       inet.Direction

	addr ma.Multiaddr
	peer peer.ID
}

// tag used in the connection manager when explicitly connecting to a peer.
const connectionManagerTag = "user-connect"
const connectionManagerWeight = 100

func (api *SwarmAPI) Connect(ctx context.Context, pi peer.AddrInfo) error {
	if api.peerHost == nil {
		return coreiface.ErrOffline
	}

	if swrm, ok := api.peerHost.Network().(*swarm.Swarm); ok {
		swrm.Backoff().Clear(pi.ID)
	}

	if err := api.peerHost.Connect(ctx, pi); err != nil {
		return err
	}

	api.peerHost.ConnManager().TagPeer(pi.ID, connectionManagerTag, connectionManagerWeight)
	return nil
}

func (api *SwarmAPI) Disconnect(ctx context.Context, addr ma.Multiaddr) error {
	if api.peerHost == nil {
		return coreiface.ErrOffline
	}

	taddr, id := peer.SplitAddr(addr)
	if id == "" {
		return peer.ErrInvalidAddr
	}

	net := api.peerHost.Network()
	if taddr == nil {
		if net.Connectedness(id) != inet.Connected {
			return coreiface.ErrNotConnected
		}
		if err := net.ClosePeer(id); err != nil {
			return err
		}
		return nil
	}
	for _, conn := range net.ConnsToPeer(id) {
		if !conn.RemoteMultiaddr().Equal(taddr) {
			continue
		}

		return conn.Close()
	}
	return coreiface.ErrConnNotFound
}

func (api *SwarmAPI) KnownAddrs(context.Context) (map[peer.ID][]ma.Multiaddr, error) {
	if api.peerHost == nil {
		return nil, coreiface.ErrOffline
	}

	addrs := make(map[peer.ID][]ma.Multiaddr)
	ps := api.peerHost.Network().Peerstore()
	for _, p := range ps.Peers() {
		addrs[p] = append(addrs[p], ps.Addrs(p)...)
		sort.Slice(addrs[p], func(i, j int) bool {
			return addrs[p][i].String() < addrs[p][j].String()
		})
	}

	return addrs, nil
}

func (api *SwarmAPI) LocalAddrs(context.Context) ([]ma.Multiaddr, error) {
	if api.peerHost == nil {
		return nil, coreiface.ErrOffline
	}

	return api.peerHost.Addrs(), nil
}

func (api *SwarmAPI) ListenAddrs(context.Context) ([]ma.Multiaddr, error) {
	if api.peerHost == nil {
		return nil, coreiface.ErrOffline
	}

	return api.peerHost.Network().InterfaceListenAddresses()
}

func (api *SwarmAPI) Peers(context.Context) ([]coreiface.ConnectionInfo, error) {
	if api.peerHost == nil {
		return nil, coreiface.ErrOffline
	}

	conns := api.peerHost.Network().Conns()

	out := make([]coreiface.ConnectionInfo, 0, len(conns))
	for _, c := range conns {

		ci := &connInfo{
			peerstore: api.peerstore,
			conn:      c,
			dir:       c.Stat().Direction,

			addr: c.RemoteMultiaddr(),
			peer: c.RemotePeer(),
		}

		/*
			// FIXME(steb):
			swcon, ok := c.(*swarm.Conn)
			if ok {
				ci.muxer = fmt.Sprintf("%T", swcon.StreamConn().Conn())
			}
		*/

		out = append(out, ci)
	}

	return out, nil
}

func (ci *connInfo) ID() peer.ID {
	return ci.peer
}

func (ci *connInfo) Address() ma.Multiaddr {
	return ci.addr
}

func (ci *connInfo) Direction() inet.Direction {
	return ci.dir
}

func (ci *connInfo) Latency() (time.Duration, error) {
	return ci.peerstore.LatencyEWMA(peer.ID(ci.ID())), nil
}

func (ci *connInfo) Streams() ([]protocol.ID, error) {
	streams := ci.conn.GetStreams()

	out := make([]protocol.ID, len(streams))
	for i, s := range streams {
		out[i] = s.Protocol()
	}

	return out, nil
}
