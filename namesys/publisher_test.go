package namesys

import (
	"context"
	"crypto/rand"
	"github.com/ipfs/go-path"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	dshelp "github.com/ipfs/go-ipfs-ds-help"
	mockrouting "github.com/ipfs/go-ipfs-routing/mock"
	ipns "github.com/ipfs/go-ipns"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	testutil "github.com/libp2p/go-libp2p-testing/net"
	ma "github.com/multiformats/go-multiaddr"
)

type identity struct {
	testutil.PeerNetParams
}

func (p *identity) ID() peer.ID {
	return p.PeerNetParams.ID
}

func (p *identity) Address() ma.Multiaddr {
	return p.Addr
}

func (p *identity) PrivateKey() ci.PrivKey {
	return p.PrivKey
}

func (p *identity) PublicKey() ci.PubKey {
	return p.PubKey
}

func testNamekeyPublisher(t *testing.T, keyType int, expectedErr error, expectedExistence bool) {
	// Context
	ctx := context.Background()

	// Private key
	privKey, pubKey, err := ci.GenerateKeyPairWithReader(keyType, 2048, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// ID
	id, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		t.Fatal(err)
	}

	// Value
	value := []byte("ipfs/TESTING")

	// Seqnum
	seqnum := uint64(0)

	// Eol
	eol := time.Now().Add(24 * time.Hour)

	// Routing value store
	p := testutil.PeerNetParams{
		ID:      id,
		PrivKey: privKey,
		PubKey:  pubKey,
		Addr:    testutil.ZeroLocalTCPAddress,
	}

	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	serv := mockrouting.NewServer()
	r := serv.ClientWithDatastore(context.Background(), &identity{p}, dstore)

	entry, err := ipns.Create(privKey, value, seqnum, eol)
	if err != nil {
		t.Fatal(err)
	}

	err = PutRecordToRouting(ctx, r, pubKey, entry)
	if err != nil {
		t.Fatal(err)
	}

	// Check for namekey existence in value store
	namekey := PkKeyForID(id)
	_, err = r.GetValue(ctx, namekey)
	if err != expectedErr {
		t.Fatal(err)
	}

	// Also check datastore for completeness
	key := dshelp.NewKeyFromBinary([]byte(namekey))
	exists, err := dstore.Has(ctx, key)
	if err != nil {
		t.Fatal(err)
	}

	if exists != expectedExistence {
		t.Fatal("Unexpected key existence in datastore")
	}
}

func TestRSAPublisher(t *testing.T) {
	testNamekeyPublisher(t, ci.RSA, nil, true)
}

func TestEd22519Publisher(t *testing.T) {
	testNamekeyPublisher(t, ci.Ed25519, ds.ErrNotFound, false)
}

func TestAsyncDS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := mockrouting.NewServer().Client(testutil.RandIdentityOrFatal(t))
	ds := &checkSyncDS{
		Datastore: ds.NewMapDatastore(),
		syncKeys:  make(map[ds.Key]struct{}),
	}
	publisher := NewIpnsPublisher(rt, ds)

	ipnsFakeID := testutil.RandIdentityOrFatal(t)
	ipnsVal, err := path.ParsePath("/ipns/foo.bar")
	if err != nil {
		t.Fatal(err)
	}

	if err := publisher.Publish(ctx, ipnsFakeID.PrivateKey(), ipnsVal); err != nil {
		t.Fatal(err)
	}

	ipnsKey := IpnsDsKey(ipnsFakeID.ID())

	for k := range ds.syncKeys {
		if k.IsAncestorOf(ipnsKey) || k.Equal(ipnsKey) {
			return
		}
	}

	t.Fatal("ipns key not synced")
}

type checkSyncDS struct {
	ds.Datastore
	syncKeys map[ds.Key]struct{}
}

func (d *checkSyncDS) Sync(ctx context.Context, prefix ds.Key) error {
	d.syncKeys[prefix] = struct{}{}
	return d.Datastore.Sync(ctx, prefix)
}
