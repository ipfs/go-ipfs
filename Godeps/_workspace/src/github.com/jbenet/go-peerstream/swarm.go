package peerstream

import (
	"errors"
	"net"
	"sync"
)

// fd is a (file) descriptor, unix style
type fd uint32

type Swarm struct {
	// active streams.
	streams    map[*Stream]struct{}
	streamLock sync.RWMutex

	// active connections. generate new Streams
	conns    map[*Conn]struct{}
	connLock sync.RWMutex

	// active listeners. generate new Listeners
	listeners    map[*Listener]struct{}
	listenerLock sync.RWMutex

	// these handlers should be accessed with their getter/setter
	// as this pointer may be changed at any time.
	handlerLock   sync.RWMutex  // protects the functions below
	connHandler   ConnHandler   // receives Conns intiated remotely
	streamHandler StreamHandler // receives Streams initiated remotely
	selectConn    SelectConn    // default SelectConn function
}

func NewSwarm() *Swarm {
	return &Swarm{
		streams:       make(map[*Stream]struct{}),
		conns:         make(map[*Conn]struct{}),
		listeners:     make(map[*Listener]struct{}),
		selectConn:    SelectRandomConn,
		streamHandler: NoOpStreamHandler,
		connHandler:   NoOpConnHandler,
	}
}

// SetStreamHandler assigns the stream handler in the swarm.
// The handler assumes responsibility for closing the stream.
// This need not happen at the end of the handler, leaving the
// stream open (to be used and closed later) is fine.
// It is also fine to keep a pointer to the Stream.
// This is a threadsafe (atomic) operation
func (s *Swarm) SetStreamHandler(sh StreamHandler) {
	s.handlerLock.Lock()
	defer s.handlerLock.Unlock()
	s.streamHandler = sh
}

// StreamHandler returns the Swarm's current StreamHandler.
// This is a threadsafe (atomic) operation
func (s *Swarm) StreamHandler() StreamHandler {
	s.handlerLock.RLock()
	defer s.handlerLock.RUnlock()
	if s.streamHandler == nil {
		return NoOpStreamHandler
	}
	return s.streamHandler
}

// SetConnHandler assigns the conn handler in the swarm.
// Unlike the StreamHandler, the ConnHandler has less respon-
// ibility for the Connection. The Swarm is still its client.
// This handler is only a notification.
// This is a threadsafe (atomic) operation
func (s *Swarm) SetConnHandler(ch ConnHandler) {
	s.handlerLock.Lock()
	defer s.handlerLock.Unlock()
	s.connHandler = ch
}

// ConnHandler returns the Swarm's current ConnHandler.
// This is a threadsafe (atomic) operation
func (s *Swarm) ConnHandler() ConnHandler {
	s.handlerLock.RLock()
	defer s.handlerLock.RUnlock()
	if s.connHandler == nil {
		return NoOpConnHandler
	}
	return s.connHandler
}

// SetConnSelect assigns the connection selector in the swarm.
// If cs is nil, will use SelectRandomConn
// This is a threadsafe (atomic) operation
func (s *Swarm) SetSelectConn(cs SelectConn) {
	s.handlerLock.Lock()
	defer s.handlerLock.Unlock()
	s.selectConn = cs
}

// ConnSelect returns the Swarm's current connection selector.
// ConnSelect is used in order to select the best of a set of
// possible connections. The default chooses one at random.
// This is a threadsafe (atomic) operation
func (s *Swarm) SelectConn() SelectConn {
	s.handlerLock.RLock()
	defer s.handlerLock.RUnlock()
	if s.selectConn == nil {
		return SelectRandomConn
	}
	return s.selectConn
}

// Conns returns all the connections associated with this Swarm.
func (s *Swarm) Conns() []*Conn {
	s.connLock.RLock()
	conns := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.connLock.RUnlock()
	return conns
}

// Listeners returns all the listeners associated with this Swarm.
func (s *Swarm) Listeners() []*Listener {
	s.listenerLock.RLock()
	out := make([]*Listener, 0, len(s.listeners))
	for c := range s.listeners {
		out = append(out, c)
	}
	s.listenerLock.RUnlock()
	return out
}

// Streams returns all the streams associated with this Swarm.
func (s *Swarm) Streams() []*Stream {
	s.streamLock.RLock()
	out := make([]*Stream, 0, len(s.streams))
	for c := range s.streams {
		out = append(out, c)
	}
	s.streamLock.RUnlock()
	return out
}

// AddListener adds net.Listener to the Swarm, and immediately begins
// accepting incoming connections.
func (s *Swarm) AddListener(l net.Listener) (*Listener, error) {
	return s.addListener(l)
}

// AddListenerWithRateLimit adds Listener to the Swarm, and immediately
// begins accepting incoming connections. The rate of connection acceptance
// depends on the RateLimit option
// func (s *Swarm) AddListenerWithRateLimit(net.Listner, RateLimit) // TODO

// AddConn gives the Swarm ownership of net.Conn. The Swarm will open a
// SPDY session and begin listening for Streams.
// Returns the resulting Swarm-associated peerstream.Conn.
// Idempotent: if the Connection has already been added, this is a no-op.
func (s *Swarm) AddConn(netConn net.Conn) (*Conn, error) {
	return s.addConn(netConn, false)
}

// NewStream opens a new Stream on the best available connection,
// as selected by current swarm.SelectConn.
func (s *Swarm) NewStream() (*Stream, error) {
	return s.NewStreamSelectConn(s.SelectConn())
}

func (s *Swarm) newStreamSelectConn(selConn SelectConn, conns []*Conn) (*Stream, error) {
	if selConn == nil {
		return nil, errors.New("nil SelectConn")
	}

	best := selConn(conns)
	if best == nil || !ConnInConns(best, conns) {
		return nil, ErrInvalidConnSelected
	}
	return s.NewStreamWithConn(best)
}

// NewStreamWithSelectConn opens a new Stream on a connection selected
// by selConn.
func (s *Swarm) NewStreamSelectConn(selConn SelectConn) (*Stream, error) {
	if selConn == nil {
		return nil, errors.New("nil SelectConn")
	}

	conns := s.Conns()
	if len(conns) == 0 {
		return nil, ErrNoConnections
	}
	return s.newStreamSelectConn(selConn, conns)
}

// NewStreamWithGroup opens a new Stream on an available connection in
// the given group. Uses the current swarm.SelectConn to pick between
// multiple connections.
func (s *Swarm) NewStreamWithGroup(group Group) (*Stream, error) {
	conns := s.ConnsWithGroup(group)
	return s.newStreamSelectConn(s.SelectConn(), conns)
}

// NewStreamWithNetConn opens a new Stream on given net.Conn.
// Calls s.AddConn(netConn).
func (s *Swarm) NewStreamWithNetConn(netConn net.Conn) (*Stream, error) {
	c, err := s.AddConn(netConn)
	if err != nil {
		return nil, err
	}
	return s.NewStreamWithConn(c)
}

// NewStreamWithConnection opens a new Stream on given connection.
func (s *Swarm) NewStreamWithConn(conn *Conn) (*Stream, error) {
	if conn == nil {
		return nil, errors.New("nil Conn")
	}
	if conn.Swarm() != s {
		return nil, errors.New("connection not associated with swarm")
	}

	s.connLock.RLock()
	if _, found := s.conns[conn]; !found {
		s.connLock.RUnlock()
		return nil, errors.New("connection not associated with swarm")
	}
	s.connLock.RUnlock()
	return s.createStream(conn)
}

// AddConnToGroup assigns given Group to conn
func (s *Swarm) AddConnToGroup(conn *Conn, g Group) {
	conn.groups.Add(g)
}

// ConnsWithGroup returns all the connections with a given Group
func (s *Swarm) ConnsWithGroup(g Group) []*Conn {
	return ConnsWithGroup(g, s.Conns())
}

// StreamsWithGroup returns all the streams with a given Group
func (s *Swarm) StreamsWithGroup(g Group) []*Stream {
	return StreamsWithGroup(g, s.Streams())
}

// Close shuts down the Swarm, and it's listeners.
func (s *Swarm) Close() error {
	// shut down TODO
	return nil
}
