package mockrouting

import (
	"testing"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	delay "github.com/jbenet/go-ipfs/util/delay"
)

func TestKeyNotFound(t *testing.T) {

	var pi = peer.PeerInfo{ID: peer.ID("the peer id")}
	var key = u.Key("mock key")
	var ctx = context.Background()

	rs := NewServer()
	providers := rs.Client(pi).FindProvidersAsync(ctx, key, 10)
	_, ok := <-providers
	if ok {
		t.Fatal("should be closed")
	}
}

func TestClientFindProviders(t *testing.T) {
	pi := peer.PeerInfo{ID: peer.ID("42")}
	rs := NewServer()
	client := rs.Client(pi)

	k := u.Key("hello")
	err := client.Provide(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}

	// This is bad... but simulating networks is hard
	time.Sleep(time.Millisecond * 300)
	max := 100

	providersFromHashTable, err := rs.Client(pi).FindProviders(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}

	isInHT := false
	for _, pi := range providersFromHashTable {
		if pi.ID == pi.ID {
			isInHT = true
		}
	}
	if !isInHT {
		t.Fatal("Despite client providing key, peer wasn't in hash table as a provider")
	}
	providersFromClient := client.FindProvidersAsync(context.Background(), u.Key("hello"), max)
	isInClient := false
	for pi := range providersFromClient {
		if pi.ID == pi.ID {
			isInClient = true
		}
	}
	if !isInClient {
		t.Fatal("Despite client providing key, client didn't receive peer when finding providers")
	}
}

func TestClientOverMax(t *testing.T) {
	rs := NewServer()
	k := u.Key("hello")
	numProvidersForHelloKey := 100
	for i := 0; i < numProvidersForHelloKey; i++ {
		pi := peer.PeerInfo{ID: peer.ID(i)}
		err := rs.Client(pi).Provide(context.Background(), k)
		if err != nil {
			t.Fatal(err)
		}
	}

	max := 10
	pi := peer.PeerInfo{ID: peer.ID("TODO")}
	client := rs.Client(pi)

	providersFromClient := client.FindProvidersAsync(context.Background(), k, max)
	i := 0
	for _ = range providersFromClient {
		i++
	}
	if i != max {
		t.Fatal("Too many providers returned")
	}
}

// TODO does dht ensure won't receive self as a provider? probably not.
func TestCanceledContext(t *testing.T) {
	rs := NewServer()
	k := u.Key("hello")

	// avoid leaking goroutine, without using the context to signal
	// (we want the goroutine to keep trying to publish on a
	// cancelled context until we've tested it doesnt do anything.)
	done := make(chan struct{})
	defer func() { done <- struct{}{} }()

	t.Log("async'ly announce infinite stream of providers for key")
	i := 0
	go func() { // infinite stream
		for {
			select {
			case <-done:
				t.Log("exiting async worker")
				return
			default:
			}

			pi := peer.PeerInfo{ID: peer.ID(i)}
			err := rs.Client(pi).Provide(context.Background(), k)
			if err != nil {
				t.Error(err)
			}
			i++
		}
	}()

	local := peer.PeerInfo{ID: peer.ID("peer id doesn't matter")}
	client := rs.Client(local)

	t.Log("warning: max is finite so this test is non-deterministic")
	t.Log("context cancellation could simply take lower priority")
	t.Log("and result in receiving the max number of results")
	max := 1000

	t.Log("cancel the context before consuming")
	ctx, cancelFunc := context.WithCancel(context.Background())
	cancelFunc()
	providers := client.FindProvidersAsync(ctx, k, max)

	numProvidersReturned := 0
	for _ = range providers {
		numProvidersReturned++
	}
	t.Log(numProvidersReturned)

	if numProvidersReturned == max {
		t.Fatal("Context cancel had no effect")
	}
}

func TestValidAfter(t *testing.T) {

	var pi = peer.PeerInfo{ID: peer.ID("the peer id")}
	var key = u.Key("mock key")
	var ctx = context.Background()
	conf := DelayConfig{
		ValueVisibility: delay.Fixed(1 * time.Hour),
		Query:           delay.Fixed(0),
	}

	rs := NewServerWithDelay(conf)

	rs.Client(pi).Provide(ctx, key)

	var providers []peer.PeerInfo
	providers, err := rs.Client(pi).FindProviders(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) > 0 {
		t.Fail()
	}

	conf.ValueVisibility.Set(0)
	providers, err = rs.Client(pi).FindProviders(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("providers", providers)
	if len(providers) != 1 {
		t.Fail()
	}
}
