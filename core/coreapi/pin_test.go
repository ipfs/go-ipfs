package coreapi_test

import (
	"context"
	"strings"
	"testing"

	opt "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

func TestPinAdd(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	p, err := api.Unixfs().Add(ctx, strings.NewReader("foo"))
	if err != nil {
		t.Error(err)
	}

	err = api.Pin().Add(ctx, p)
	if err != nil {
		t.Error(err)
	}
}

func TestPinSimple(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	p, err := api.Unixfs().Add(ctx, strings.NewReader("foo"))
	if err != nil {
		t.Error(err)
	}

	err = api.Pin().Add(ctx, p)
	if err != nil {
		t.Error(err)
	}

	list, err := api.Pin().Ls(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	if list[0].Path().Cid().String() != p.Cid().String() {
		t.Error("paths don't match")
	}

	if list[0].Type() != "recursive" {
		t.Error("unexpected pin type")
	}

	err = api.Pin().Rm(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	list, err = api.Pin().Ls(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 0 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}
}

func TestPinRecursive(t *testing.T) {
	ctx := context.Background()
	nd, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	p0, err := api.Unixfs().Add(ctx, strings.NewReader("foo"))
	if err != nil {
		t.Error(err)
	}

	p1, err := api.Unixfs().Add(ctx, strings.NewReader("bar"))
	if err != nil {
		t.Error(err)
	}

	p2, err := api.Dag().Put(ctx, strings.NewReader(`{"lnk": {"/": "`+p0.Cid().String()+`"}}`))
	if err != nil {
		t.Error(err)
	}

	p3, err := api.Dag().Put(ctx, strings.NewReader(`{"lnk": {"/": "`+p1.Cid().String()+`"}}`))
	if err != nil {
		t.Error(err)
	}

	err = api.Pin().Add(ctx, p2)
	if err != nil {
		t.Error(err)
	}

	err = api.Pin().Add(ctx, p3, opt.Pin.Recursive(false))
	if err != nil {
		t.Error(err)
	}

	list, err := api.Pin().Ls(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 3 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	list, err = api.Pin().Ls(ctx, opt.Pin.Type.Direct())
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	if list[0].Path().String() != p3.String() {
		t.Error("unexpected path")
	}

	list, err = api.Pin().Ls(ctx, opt.Pin.Type.Recursive())
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	if list[0].Path().String() != p2.String() {
		t.Error("unexpected path")
	}

	list, err = api.Pin().Ls(ctx, opt.Pin.Type.Indirect())
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	if list[0].Path().Cid().String() != p0.Cid().String() {
		t.Error("unexpected path")
	}

	res, err := api.Pin().Verify(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for r := range res {
		if !r.Ok() {
			t.Error("expected pin to be ok")
		}
		n++
	}

	if n != 1 {
		t.Errorf("unexpected verify result count: %d", n)
	}

	err = nd.Blockstore.Delete(p0.Cid().Hash())
	if err != nil {
		t.Fatal(err)
	}

	res, err = api.Pin().Verify(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n = 0
	for r := range res {
		if r.Ok() {
			t.Error("expected pin to not be ok")
		}

		if len(r.BadNodes()) != 1 {
			t.Fatalf("unexpected badNodes len")
		}

		if r.BadNodes()[0].Path().Cid().String() != p0.Cid().String() {
			t.Error("unexpected badNode path")
		}

		if r.BadNodes()[0].Err().Error() != "merkledag: not found" {
			t.Errorf("unexpected badNode error: %s", r.BadNodes()[0].Err().Error())
		}
		n++
	}

	if n != 1 {
		t.Errorf("unexpected verify result count: %d", n)
	}
}
