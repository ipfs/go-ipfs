package decision

import (
	"math"
	"testing"

	key "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	"github.com/ipfs/go-ipfs/thirdparty/testutil"
	"gx/ipfs/QmR7tPYgkwZym7WLVLdhYr3jMnhWMtD2ovxosofpiU3BqZ/go-libp2p/p2p/peer"
)

// FWIW: At the time of this commit, including a timestamp in task increases
// time cost of Push by 3%.
func BenchmarkTaskQueuePush(b *testing.B) {
	q := newPRQ()
	peers := []peer.ID{
		testutil.RandPeerIDFatal(b),
		testutil.RandPeerIDFatal(b),
		testutil.RandPeerIDFatal(b),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push(wantlist.Entry{Key: key.Key(i), Priority: math.MaxInt32}, peers[i%len(peers)])
	}
}
