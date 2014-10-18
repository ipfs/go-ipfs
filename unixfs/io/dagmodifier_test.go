package io

import (
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/op/go-logging"
	bs "github.com/jbenet/go-ipfs/blockservice"
	"github.com/jbenet/go-ipfs/importer/chunk"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
	u "github.com/jbenet/go-ipfs/util"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
)

func getMockDagServ(t *testing.T) mdag.DAGService {
	dstore := ds.NewMapDatastore()
	bserv, err := bs.NewBlockService(dstore, nil)
	if err != nil {
		t.Fatal(err)
	}
	return mdag.NewDAGService(bserv)
}

func getNode(t *testing.T, dserv mdag.DAGService, size int64) ([]byte, *mdag.Node) {
	dw := NewDagWriter(dserv, chunk.NewSizeSplitter(500))

	n, err := io.CopyN(dw, u.NewFastRand(), size)
	if err != nil {
		t.Fatal(err)
	}
	if n != size {
		t.Fatal("Incorrect copy amount!")
	}

	err = dw.Close()
	if err != nil {
		t.Fatal("DagWriter failed to close,", err)
	}

	node := dw.GetNode()

	dr, err := NewDagReader(node, dserv)
	if err != nil {
		t.Fatal(err)
	}

	b, err := ioutil.ReadAll(dr)
	if err != nil {
		t.Fatal(err)
	}

	return b, node
}

func testModWrite(t *testing.T, beg, size uint64, orig []byte, dm *DagModifier) []byte {
	newdata := make([]byte, size)
	r := u.NewFastRand()
	r.Read(newdata)

	if size+beg > uint64(len(orig)) {
		orig = append(orig, make([]byte, (size+beg)-uint64(len(orig)))...)
	}
	copy(orig[beg:], newdata)

	nmod, err := dm.WriteAt(newdata, uint64(beg))
	if err != nil {
		t.Fatal(err)
	}

	if nmod != int(size) {
		t.Fatalf("Mod length not correct! %d != %d", nmod, size)
	}

	nd, err := dm.GetNode()
	if err != nil {
		t.Fatal(err)
	}

	rd, err := NewDagReader(nd, dm.dagserv)
	if err != nil {
		t.Fatal(err)
	}

	after, err := ioutil.ReadAll(rd)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(after, orig)
	if err != nil {
		t.Fatal(err)
	}
	return orig
}

func TestDagModifierBasic(t *testing.T) {
	logging.SetLevel(logging.CRITICAL, "blockservice")
	logging.SetLevel(logging.CRITICAL, "merkledag")
	dserv := getMockDagServ(t)
	b, n := getNode(t, dserv, 50000)

	dagmod, err := NewDagModifier(n, dserv, chunk.NewSizeSplitter(512))
	if err != nil {
		t.Fatal(err)
	}

	// Within zero block
	beg := uint64(15)
	length := uint64(60)

	t.Log("Testing mod within zero block")
	b = testModWrite(t, beg, length, b, dagmod)

	// Within bounds of existing file
	beg = 1000
	length = 4000
	t.Log("Testing mod within bounds of existing file.")
	b = testModWrite(t, beg, length, b, dagmod)

	// Extend bounds
	beg = 49500
	length = 4000

	t.Log("Testing mod that extends file.")
	b = testModWrite(t, beg, length, b, dagmod)

	// "Append"
	beg = uint64(len(b))
	length = 3000
	b = testModWrite(t, beg, length, b, dagmod)

	// Verify reported length
	node, err := dagmod.GetNode()
	if err != nil {
		t.Fatal(err)
	}

	size, err := ft.DataSize(node.Data)
	if err != nil {
		t.Fatal(err)
	}

	expected := uint64(50000 + 3500 + 3000)
	if size != expected {
		t.Fatal("Final reported size is incorrect [%d != %d]", size, expected)
	}
}

func TestMultiWrite(t *testing.T) {
	dserv := getMockDagServ(t)
	_, n := getNode(t, dserv, 0)

	dagmod, err := NewDagModifier(n, dserv, chunk.NewSizeSplitter(512))
	if err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 4000)
	u.NewFastRand().Read(data)

	for i := 0; i < len(data); i++ {
		n, err := dagmod.WriteAt(data[i:i+1], uint64(i))
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatal("Somehow wrote the wrong number of bytes! (n != 1)")
		}
	}
	nd, err := dagmod.GetNode()
	if err != nil {
		t.Fatal(err)
	}

	read, err := NewDagReader(nd, dserv)
	if err != nil {
		t.Fatal(err)
	}
	rbuf, err := ioutil.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(rbuf, data)
	if err != nil {
		t.Fatal(err)
	}
}

func arrComp(a, b []byte) error {
	if len(a) != len(b) {
		return fmt.Errorf("Arrays differ in length. %d != %d", len(a), len(b))
	}
	for i, v := range a {
		if v != b[i] {
			return fmt.Errorf("Arrays differ at index: %d", i)
		}
	}
	return nil
}
