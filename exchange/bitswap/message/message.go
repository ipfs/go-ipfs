package message

import (
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	blocks "github.com/jbenet/go-ipfs/blocks"
	pb "github.com/jbenet/go-ipfs/exchange/bitswap/message/internal/pb"
	netmsg "github.com/jbenet/go-ipfs/net/message"
	nm "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// TODO move message.go into the bitswap package
// TODO move bs/msg/internal/pb to bs/internal/pb and rename pb package to bitswap_pb

type BitSwapMessage interface {
	// Wantlist returns a slice of unique keys that represent data wanted by
	// the sender.
	Wantlist() []u.Key

	// Blocks returns a slice of unique blocks
	Blocks() []*blocks.Block

	// AddWanted adds the key to the Wantlist.
	//
	// Insertion order determines priority. That is, earlier insertions are
	// deemed higher priority than keys inserted later.
	//
	// t = 0, msg.AddWanted(A)
	// t = 1, msg.AddWanted(B)
	//
	// implies Priority(A) > Priority(B)
	AddWanted(u.Key)

	AddBlock(*blocks.Block)
	Exportable
}

type Exportable interface {
	ToProto() *pb.Message
	ToNet(p peer.Peer) (nm.NetMessage, error)
}

type impl struct {
	existsInWantlist map[u.Key]struct{}      // map to detect duplicates
	wantlist         []u.Key                 // slice to preserve ordering
	blocks           map[u.Key]*blocks.Block // map to detect duplicates
}

func New() BitSwapMessage {
	return &impl{
		blocks:           make(map[u.Key]*blocks.Block),
		existsInWantlist: make(map[u.Key]struct{}),
		wantlist:         make([]u.Key, 0),
	}
}

func newMessageFromProto(pbm pb.Message) BitSwapMessage {
	m := New()
	for _, s := range pbm.GetWantlist() {
		m.AddWanted(u.Key(s))
	}
	for _, d := range pbm.GetBlocks() {
		b := blocks.NewBlock(d)
		m.AddBlock(b)
	}
	return m
}

func (m *impl) Wantlist() []u.Key {
	return m.wantlist
}

func (m *impl) Blocks() []*blocks.Block {
	bs := make([]*blocks.Block, 0)
	for _, block := range m.blocks {
		bs = append(bs, block)
	}
	return bs
}

func (m *impl) AddWanted(k u.Key) {
	_, exists := m.existsInWantlist[k]
	if exists {
		return
	}
	m.existsInWantlist[k] = struct{}{}
	m.wantlist = append(m.wantlist, k)
}

func (m *impl) AddBlock(b *blocks.Block) {
	m.blocks[b.Key()] = b
}

func FromNet(nmsg netmsg.NetMessage) (BitSwapMessage, error) {
	pb := new(pb.Message)
	if err := proto.Unmarshal(nmsg.Data(), pb); err != nil {
		return nil, err
	}
	m := newMessageFromProto(*pb)
	return m, nil
}

func (m *impl) ToProto() *pb.Message {
	pb := new(pb.Message)
	for _, k := range m.Wantlist() {
		pb.Wantlist = append(pb.Wantlist, string(k))
	}
	for _, b := range m.Blocks() {
		pb.Blocks = append(pb.Blocks, b.Data)
	}
	return pb
}

func (m *impl) ToNet(p peer.Peer) (nm.NetMessage, error) {
	return nm.FromObject(p, m.ToProto())
}
