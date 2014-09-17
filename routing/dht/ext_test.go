package dht

import (
	"testing"

	crand "crypto/rand"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	swarm "github.com/jbenet/go-ipfs/net/swarm"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	"time"
)

// fauxNet is a standin for a swarm.Network in order to more easily recreate
// different testing scenarios
type fauxNet struct {
	Chan     *swarm.Chan
	handlers []mesHandleFunc

	swarm.Network
}

// mesHandleFunc is a function that takes in outgoing messages
// and can respond to them, simulating other peers on the network.
// returning nil will chose not to respond and pass the message onto the
// next registered handler
type mesHandleFunc func(*swarm.Message) *swarm.Message

func newFauxNet() *fauxNet {
	fn := new(fauxNet)
	fn.Chan = swarm.NewChan(8)

	return fn
}

// Instead of 'Listening' Start up a goroutine that will check
// all outgoing messages against registered message handlers,
// and reply if needed
func (f *fauxNet) Listen() error {
	go func() {
		for {
			select {
			case in := <-f.Chan.Outgoing:
				for _, h := range f.handlers {
					reply := h(in)
					if reply != nil {
						f.Chan.Incoming <- reply
						break
					}
				}
			}
		}
	}()
	return nil
}

func (f *fauxNet) AddHandler(fn func(*swarm.Message) *swarm.Message) {
	f.handlers = append(f.handlers, fn)
}

func (f *fauxNet) Send(mes *swarm.Message) {
	f.Chan.Outgoing <- mes
}

func (f *fauxNet) GetErrChan() chan error {
	return f.Chan.Errors
}

func (f *fauxNet) GetChannel(t swarm.PBWrapper_MessageType) *swarm.Chan {
	return f.Chan
}

func (f *fauxNet) Connect(addr *ma.Multiaddr) (*peer.Peer, error) {
	return nil, nil
}

func (f *fauxNet) GetConnection(id peer.ID, addr *ma.Multiaddr) (*peer.Peer, error) {
	return &peer.Peer{ID: id, Addresses: []*ma.Multiaddr{addr}}, nil
}

func TestGetFailures(t *testing.T) {
	fn := newFauxNet()
	fn.Listen()

	local := new(peer.Peer)
	local.ID = peer.ID("test_peer")

	d := NewDHT(local, fn, ds.NewMapDatastore())

	other := &peer.Peer{ID: peer.ID("other_peer")}

	d.Start()

	d.Update(other)

	// This one should time out
	_, err := d.GetValue(u.Key("test"), time.Millisecond*10)
	if err != nil {
		if err != u.ErrTimeout {
			t.Fatal("Got different error than we expected.")
		}
	} else {
		t.Fatal("Did not get expected error!")
	}

	// Reply with failures to every message
	fn.AddHandler(func(mes *swarm.Message) *swarm.Message {
		pmes := new(PBDHTMessage)
		err := proto.Unmarshal(mes.Data, pmes)
		if err != nil {
			t.Fatal(err)
		}

		resp := Message{
			Type:     pmes.GetType(),
			ID:       pmes.GetId(),
			Response: true,
			Success:  false,
		}
		return swarm.NewMessage(mes.Peer, resp.ToProtobuf())
	})

	// This one should fail with NotFound
	_, err = d.GetValue(u.Key("test"), time.Millisecond*1000)
	if err != nil {
		if err != u.ErrNotFound {
			t.Fatalf("Expected ErrNotFound, got: %s", err)
		}
	} else {
		t.Fatal("expected error, got none.")
	}

	success := make(chan struct{})
	fn.handlers = nil
	fn.AddHandler(func(mes *swarm.Message) *swarm.Message {
		resp := new(PBDHTMessage)
		err := proto.Unmarshal(mes.Data, resp)
		if err != nil {
			t.Fatal(err)
		}
		if resp.GetSuccess() {
			t.Fatal("Get returned success when it shouldnt have.")
		}
		success <- struct{}{}
		return nil
	})

	// Now we test this DHT's handleGetValue failure
	req := Message{
		Type:  PBDHTMessage_GET_VALUE,
		Key:   "hello",
		ID:    swarm.GenerateMessageID(),
		Value: []byte{0},
	}
	fn.Chan.Incoming <- swarm.NewMessage(other, req.ToProtobuf())

	<-success
}

// TODO: Maybe put these in some sort of "ipfs_testutil" package
func _randPeer() *peer.Peer {
	p := new(peer.Peer)
	p.ID = make(peer.ID, 16)
	p.Addresses = []*ma.Multiaddr{nil}
	crand.Read(p.ID)
	return p
}

func TestNotFound(t *testing.T) {
	fn := newFauxNet()
	fn.Listen()

	local := new(peer.Peer)
	local.ID = peer.ID("test_peer")

	d := NewDHT(local, fn, ds.NewMapDatastore())
	d.Start()

	var ps []*peer.Peer
	for i := 0; i < 5; i++ {
		ps = append(ps, _randPeer())
		d.Update(ps[i])
	}

	// Reply with random peers to every message
	fn.AddHandler(func(mes *swarm.Message) *swarm.Message {
		pmes := new(PBDHTMessage)
		err := proto.Unmarshal(mes.Data, pmes)
		if err != nil {
			t.Fatal(err)
		}

		switch pmes.GetType() {
		case PBDHTMessage_GET_VALUE:
			resp := Message{
				Type:     pmes.GetType(),
				ID:       pmes.GetId(),
				Response: true,
				Success:  false,
			}

			for i := 0; i < 7; i++ {
				resp.Peers = append(resp.Peers, _randPeer())
			}
			return swarm.NewMessage(mes.Peer, resp.ToProtobuf())
		default:
			panic("Shouldnt recieve this.")
		}

	})

	_, err := d.GetValue(u.Key("hello"), time.Second*30)
	if err != nil {
		switch err {
		case u.ErrNotFound:
			//Success!
			return
		case u.ErrTimeout:
			t.Fatal("Should not have gotten timeout!")
		default:
			t.Fatalf("Got unexpected error: %s", err)
		}
	}
	t.Fatal("Expected to recieve an error.")
}

// If less than K nodes are in the entire network, it should fail when we make
// a GET rpc and nobody has the value
func TestLessThanKResponses(t *testing.T) {
	u.Debug = false
	fn := newFauxNet()
	fn.Listen()

	local := new(peer.Peer)
	local.ID = peer.ID("test_peer")

	d := NewDHT(local, fn, ds.NewMapDatastore())
	d.Start()

	var ps []*peer.Peer
	for i := 0; i < 5; i++ {
		ps = append(ps, _randPeer())
		d.Update(ps[i])
	}
	other := _randPeer()

	// Reply with random peers to every message
	fn.AddHandler(func(mes *swarm.Message) *swarm.Message {
		pmes := new(PBDHTMessage)
		err := proto.Unmarshal(mes.Data, pmes)
		if err != nil {
			t.Fatal(err)
		}

		switch pmes.GetType() {
		case PBDHTMessage_GET_VALUE:
			resp := Message{
				Type:     pmes.GetType(),
				ID:       pmes.GetId(),
				Response: true,
				Success:  false,
				Peers:    []*peer.Peer{other},
			}

			return swarm.NewMessage(mes.Peer, resp.ToProtobuf())
		default:
			panic("Shouldnt recieve this.")
		}

	})

	_, err := d.GetValue(u.Key("hello"), time.Second*30)
	if err != nil {
		switch err {
		case u.ErrNotFound:
			//Success!
			return
		case u.ErrTimeout:
			t.Fatal("Should not have gotten timeout!")
		default:
			t.Fatalf("Got unexpected error: %s", err)
		}
	}
	t.Fatal("Expected to recieve an error.")
}
