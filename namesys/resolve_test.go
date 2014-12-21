package namesys

import (
	"testing"

	ci "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"
	mockrouting "github.com/jbenet/go-ipfs/routing/mock"
	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestRoutingResolve(t *testing.T) {
	local, err := testutil.RandPeerID()
	if err != nil {
		t.Fatal(err)
	}
	d := mockrouting.NewServer().Client(peer.PeerInfo{ID: local})

	resolver := NewRoutingResolver(d)
	publisher := NewRoutingPublisher(d)

	privk, pubk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		t.Fatal(err)
	}

	err = publisher.Publish(privk, "Hello")
	if err == nil {
		t.Fatal("should have errored out when publishing a non-multihash val")
	}

	h := u.Key(u.Hash([]byte("Hello"))).Pretty()
	err = publisher.Publish(privk, h)
	if err != nil {
		t.Fatal(err)
	}

	pubkb, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	pkhash := u.Hash(pubkb)
	res, err := resolver.Resolve(u.Key(pkhash).Pretty())
	if err != nil {
		t.Fatal(err)
	}

	if res != h {
		t.Fatal("Got back incorrect value.")
	}
}
