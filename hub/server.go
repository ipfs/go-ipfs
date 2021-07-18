package hub

import (
	"context"
	"fmt"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"sync"
	"time"
)

var log = logging.Logger("pubsubcrserver")

type Server struct {
	sub *pubsub.Subscription
	bs  blockstore.Blockstore
	h   host.Host

	lifecycleMx sync.Mutex
	closingMx   sync.Mutex
	cancel      context.CancelFunc
	closing     bool
	closed      chan struct{}
	running     bool
}

func NewServer(sub *pubsub.Subscription, bs blockstore.Blockstore, h host.Host) (*Server, error) {
	return &Server{
		sub: sub,
		bs:  bs,
		h:   h,
	}, nil
}

func (s *Server) Start() error {
	s.lifecycleMx.Lock()
	defer s.lifecycleMx.Unlock()

	if s.running {
		return fmt.Errorf("cannot start while already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.closing = false
	s.running = true

	go s.start(ctx)
	return nil
}

func (s *Server) start(ctx context.Context) {
	defer func() { close(s.closed) }()
	for {
		m, err := s.sub.Next(ctx)
		if err != nil {
			s.closingMx.Lock()
			if s.closing {
				s.closingMx.Unlock()
				break
			}
			s.closingMx.Unlock()
			log.Error(err)
			continue
		}
		c, err := cid.Cast(m.GetData())
		if err != nil {
			log.Debugw("invalid data received: %w", err)
			continue
		}

		found, err := s.bs.Has(c)
		if err != nil {
			log.Errorw("blockstore error: %w", err)
			continue
		}

		if found {
			ctx, cancel := context.WithTimeout(ctx, time.Second*5)
			fromPeer, err := peer.IDFromBytes(m.From)
			if err != nil {
				log.Debugw("invalid")
			} else {
				s.h.Connect(ctx, peer.AddrInfo{ID: fromPeer})
			}
			cancel()
		}
	}
}

func (s *Server) Close() error {
	s.lifecycleMx.Lock()
	defer s.lifecycleMx.Unlock()

	if s.closing {
		return fmt.Errorf("cannot close while already closing or closed")
	}

	s.closingMx.Lock()
	s.closing = true
	s.closed = make(chan struct{})
	s.cancel()
	s.sub.Cancel()
	s.closingMx.Unlock()

	<-s.closed
	s.running = false
	s.closed = nil
	s.cancel = nil

	return nil
}
