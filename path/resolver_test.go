package path

import (
	"context"
	"fmt"
	"testing"

	"github.com/ipfs/go-ipfs/merkledag"
	dagmock "github.com/ipfs/go-ipfs/merkledag/test"

	node "gx/ipfs/QmPN7cwmpcc4DWXb4KTB9dNAJgjuPY69h3npsMfhRrQL9c/go-ipld-format"
	"gx/ipfs/QmSU6eubNdhXjFBJBSksTp8kv8YRub8mGAPv8tVJHmL2EU/go-ipfs-util"
)

func randNode() *merkledag.ProtoNode {
	nd := new(merkledag.ProtoNode)
	nd.SetData(make([]byte, 32))
	util.NewTimeSeededRand().Read(nd.Data())
	return nd
}

func TestResolveToLastNode(t *testing.T) {
	ctx := context.Background()
	dagService := dagmock.Mock()

	a := randNode()
	b := randNode()
	c := randNode()

	err := b.AddNodeLink("grandchild", c)
	if err != nil {
		t.Fatal(err)
	}

	err = a.AddNodeLink("child", b)
	if err != nil {
		t.Fatal(err)
	}

	for _, n := range []node.Node{a, b, c} {
		_, err = dagService.Add(n)
		if err != nil {
			t.Fatal(err)
		}
	}

	pth, err := FromSegments("/ipfs/", a.Cid().String(), "child", "grandchild")
	if err != nil {
		t.Fatal(err)
	}

	resolver := NewBasicResolver(dagService)
	nd, pathSegments, err := resolver.ResolveToLastNode(ctx, pth)
	if err != nil {
		log.Fatal(err)
	}

	if pathSegments != nil {
		t.Fatal(fmt.Errorf(
			"node resolution failed for %s: non-nil path segment %v",
			pth.String(), pathSegments,
		))
	}

	cKey := c.Cid()
	key := nd.Cid()
	if key.String() != cKey.String() {
		t.Fatal(fmt.Errorf(
			"node resolution failed for %s: %s != %s",
			pth.String(), key.String(), cKey.String(),
		))
	}
}

func TestRecursivePathResolution(t *testing.T) {
	ctx := context.Background()
	dagService := dagmock.Mock()

	a := randNode()
	b := randNode()
	c := randNode()

	err := b.AddNodeLink("grandchild", c)
	if err != nil {
		t.Fatal(err)
	}

	err = a.AddNodeLink("child", b)
	if err != nil {
		t.Fatal(err)
	}

	for _, n := range []node.Node{a, b, c} {
		_, err = dagService.Add(n)
		if err != nil {
			t.Fatal(err)
		}
	}

	pth, err := FromSegments("/ipfs/", a.Cid().String(), "child", "grandchild")
	if err != nil {
		t.Fatal(err)
	}

	resolver := NewBasicResolver(dagService)
	nd, err := resolver.ResolvePath(ctx, pth)
	if err != nil {
		t.Fatal(err)
	}

	cKey := c.Cid()
	key := nd.Cid()
	if key.String() != cKey.String() {
		t.Fatal(fmt.Errorf(
			"recursive path resolution failed for %s: %s != %s",
			pth.String(), key.String(), cKey.String(),
		))
	}
}
