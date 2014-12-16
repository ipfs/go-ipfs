package net

import (
	"errors"
	"fmt"
	"io"
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"
)

var log = eventlog.Logger("mux2")

// Mux provides simple stream multixplexing.
// It helps you precisely when:
//  * You have many streams
//  * You have function handlers
//
// It contains the handlers for each protocol accepted.
// It dispatches handlers for streams opened by remote peers.
//
// We use a totally ad-hoc encoding:
//   <1 byte length in bytes><string name>
// So "bitswap" is 0x0762697473776170
//
// NOTE: only the dialer specifies this muxing line.
// This is because we're using Streams :)
//
// WARNING: this datastructure IS NOT threadsafe.
// do not modify it once the network is using it.
type Mux struct {
	Default  StreamHandler // handles unknown protocols.
	Handlers StreamHandlerMap

	sync.RWMutex
}

// ReadProtocolHeader reads the stream and returns the next Handler function
// according to the muxer encoding.
func (m *Mux) ReadProtocolHeader(s io.Reader) (string, StreamHandler, error) {
	name, err := ReadLengthPrefix(s)
	if err != nil {
		return "", nil, err
	}

	m.RLock()
	h, found := m.Handlers[ProtocolID(name)]
	m.RUnlock()

	switch {
	case !found && m.Default != nil:
		return name, m.Default, nil
	case !found && m.Default == nil:
		return name, nil, errors.New("no handler with name: " + name)
	default:
		return name, h, nil
	}
}

// SetHandler sets the protocol handler on the Network's Muxer.
// This operation is threadsafe.
func (m *Mux) SetHandler(p ProtocolID, h StreamHandler) {
	m.Lock()
	m.Handlers[p] = h
	m.Unlock()
}

// Handle reads the next name off the Stream, and calls a function
func (m *Mux) Handle(s Stream) {
	go func() {
		ctx := context.Background()

		name, handler, err := m.ReadProtocolHeader(s)
		if err != nil {
			err = fmt.Errorf("protocol mux error: %s", err)
			log.Error(err)
			log.Event(ctx, "muxError", lgbl.Error(err))
			return
		}

		log.Info("muxer handle protocol: %s", name)
		log.Event(ctx, "muxHandle", eventlog.Metadata{"protocol": name})
		handler(s)
	}()
}

// ReadLengthPrefix reads the name from Reader with a length-byte-prefix.
func ReadLengthPrefix(r io.Reader) (string, error) {
	// c-string identifier
	// the first byte is our length
	l := make([]byte, 1)
	if _, err := io.ReadFull(r, l); err != nil {
		return "", err
	}
	length := int(l[0])

	// the next are our identifier
	name := make([]byte, length)
	if _, err := io.ReadFull(r, name); err != nil {
		return "", err
	}

	return string(name), nil
}

// WriteLengthPrefix writes the name into Writer with a length-byte-prefix.
func WriteLengthPrefix(w io.Writer, name string) error {
	s := make([]byte, len(name)+1)
	s[0] = byte(len(name))
	copy(s[1:], []byte(name))

	_, err := w.Write(s)
	return err
}
