package mocknet

import (
	"container/list"
	"sync"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// conn represents one side's perspective of a
// live connection between two peers.
// it goes over a particular link.
type conn struct {
	local   peer.ID
	remote  peer.ID
	net     *peernet
	link    *link
	rconn   *conn // counterpart
	streams list.List

	sync.RWMutex
}

func (c *conn) Close() error {
	for _, s := range c.allStreams() {
		s.Close()
	}
	c.net.removeConn(c)
	return nil
}

func (c *conn) addStream(s *stream) {
	c.Lock()
	s.conn = c
	c.streams.PushBack(s)
	c.Unlock()
}

func (c *conn) removeStream(s *stream) {
	c.Lock()
	defer c.Unlock()
	for e := c.streams.Front(); e != nil; e = e.Next() {
		if s == e.Value {
			c.streams.Remove(e)
			return
		}
	}
}

func (c *conn) allStreams() []inet.Stream {
	c.RLock()
	defer c.RUnlock()

	strs := make([]inet.Stream, 0, c.streams.Len())
	for e := c.streams.Front(); e != nil; e = e.Next() {
		s := e.Value.(*stream)
		strs = append(strs, s)
	}
	return strs
}

func (c *conn) remoteOpenedStream(s *stream) {
	c.addStream(s)
	c.net.handleNewStream(s)
}

func (c *conn) openStream() *stream {
	sl, sr := c.link.newStreamPair()
	c.addStream(sl)
	c.rconn.remoteOpenedStream(sr)
	return sl
}

func (c *conn) NewStreamWithProtocol(pr inet.ProtocolID, p peer.ID) (inet.Stream, error) {
	log.Debugf("Conn.NewStreamWithProtocol: %s --> %s", c.local, p)

	s := c.openStream()
	if err := inet.WriteProtocolHeader(pr, s); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// LocalMultiaddr is the Multiaddr on this side
func (c *conn) LocalMultiaddr() ma.Multiaddr {
	return nil
}

// LocalPeer is the Peer on our side of the connection
func (c *conn) LocalPeer() peer.ID {
	return c.local
}

// RemoteMultiaddr is the Multiaddr on the remote side
func (c *conn) RemoteMultiaddr() ma.Multiaddr {
	return nil
}

// RemotePeer is the Peer on the remote side
func (c *conn) RemotePeer() peer.ID {
	return c.remote
}
