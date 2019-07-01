package coreunix_test

import (
	"bytes"
	"context"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/pin/gc"
	"github.com/ipfs/go-ipfs/repo"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	config "github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	pi "github.com/ipfs/go-ipfs-posinfo"
	dag "github.com/ipfs/go-merkledag"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

const testPeerID = "QmTFauExutTsy4XP6JbMFcw2Wa9645HJt2bTqL6qYDCKfe"

func TestAddMultipleGCLive(t *testing.T) {
	r := &repo.Mock{
		C: config.Config{
			Identity: config.Identity{
				PeerID: testPeerID, // required by offline node
			},
		},
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
	}
	api, err := coreapi.New(
		coreapi.Ctx(context.Background()),

		coreapi.Repo(r),
	)
	if err != nil {
		t.Fatal(err)
	}
	// nolint
	node := api.Node()

	out := make(chan interface{}, 10)
	adder, err := coreunix.NewAdder(context.Background(), node.Pinning, node.Blockstore, node.DAG)
	if err != nil {
		t.Fatal(err)
	}
	adder.Out = out

	// make two files with pipes so we can 'pause' the add for timing of the test
	piper1, pipew1 := io.Pipe()
	hangfile1 := files.NewReaderFile(piper1)

	piper2, pipew2 := io.Pipe()
	hangfile2 := files.NewReaderFile(piper2)

	rfc := files.NewBytesFile([]byte("testfileA"))

	slf := files.NewMapDirectory(map[string]files.Node{
		"a": hangfile1,
		"b": hangfile2,
		"c": rfc,
	})

	go func() {
		defer close(out)
		_, _ = adder.AddAllAndPin(slf)
		// Ignore errors for clarity - the real bug would be gc'ing files while adding them, not this resultant error
	}()

	// Start writing the first file but don't close the stream
	if _, err := pipew1.Write([]byte("some data for file a")); err != nil {
		t.Fatal(err)
	}

	var gc1out <-chan gc.Result
	gc1started := make(chan struct{})
	go func() {
		defer close(gc1started)
		gc1out = gc.GC(context.Background(), node.Blockstore, node.Repo.Datastore(), node.Pinning, nil)
	}()

	// GC shouldn't get the lock until after the file is completely added
	select {
	case <-gc1started:
		t.Fatal("gc shouldnt have started yet")
	default:
	}

	// finish write and unblock gc
	pipew1.Close()

	// Should have gotten the lock at this point
	<-gc1started

	removedHashes := make(map[string]struct{})
	for r := range gc1out {
		if r.Error != nil {
			t.Fatal(err)
		}
		removedHashes[r.KeyRemoved.String()] = struct{}{}
	}

	if _, err := pipew2.Write([]byte("some data for file b")); err != nil {
		t.Fatal(err)
	}

	var gc2out <-chan gc.Result
	gc2started := make(chan struct{})
	go func() {
		defer close(gc2started)
		gc2out = gc.GC(context.Background(), node.Blockstore, node.Repo.Datastore(), node.Pinning, nil)
	}()

	select {
	case <-gc2started:
		t.Fatal("gc shouldnt have started yet")
	default:
	}

	pipew2.Close()

	<-gc2started

	for r := range gc2out {
		if r.Error != nil {
			t.Fatal(err)
		}
		removedHashes[r.KeyRemoved.String()] = struct{}{}
	}

	for o := range out {
		if _, ok := removedHashes[o.(*coreiface.AddEvent).Path.Cid().String()]; ok {
			t.Fatal("gc'ed a hash we just added")
		}
	}
}

func TestAddGCLive(t *testing.T) {
	r := &repo.Mock{
		C: config.Config{
			Identity: config.Identity{
				PeerID: testPeerID, // required by offline node
			},
		},
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
	}
	api, err := coreapi.New(
		coreapi.Ctx(context.Background()),

		coreapi.Repo(r),
	)
	if err != nil {
		t.Fatal(err)
	}
	// nolint
	node := api.Node()

	out := make(chan interface{})
	adder, err := coreunix.NewAdder(context.Background(), node.Pinning, node.Blockstore, node.DAG)
	if err != nil {
		t.Fatal(err)
	}
	adder.Out = out

	rfa := files.NewBytesFile([]byte("testfileA"))

	// make two files with pipes so we can 'pause' the add for timing of the test
	piper, pipew := io.Pipe()
	hangfile := files.NewReaderFile(piper)

	rfd := files.NewBytesFile([]byte("testfileD"))

	slf := files.NewMapDirectory(map[string]files.Node{
		"a": rfa,
		"b": hangfile,
		"d": rfd,
	})

	addDone := make(chan struct{})
	go func() {
		defer close(addDone)
		defer close(out)
		_, err := adder.AddAllAndPin(slf)

		if err != nil {
			t.Error(err)
		}

	}()

	addedHashes := make(map[string]struct{})
	select {
	case o := <-out:
		addedHashes[o.(*coreiface.AddEvent).Path.Cid().String()] = struct{}{}
	case <-addDone:
		t.Fatal("add shouldnt complete yet")
	}

	var gcout <-chan gc.Result
	gcstarted := make(chan struct{})
	go func() {
		defer close(gcstarted)
		gcout = gc.GC(context.Background(), node.Blockstore, node.Repo.Datastore(), node.Pinning, nil)
	}()

	// gc shouldnt start until we let the add finish its current file.
	if _, err := pipew.Write([]byte("some data for file b")); err != nil {
		t.Fatal(err)
	}

	select {
	case <-gcstarted:
		t.Fatal("gc shouldnt have started yet")
	default:
	}

	time.Sleep(time.Millisecond * 100) // make sure gc gets to requesting lock

	// finish write and unblock gc
	pipew.Close()

	// receive next object from adder
	o := <-out
	addedHashes[o.(*coreiface.AddEvent).Path.Cid().String()] = struct{}{}

	<-gcstarted

	for r := range gcout {
		if r.Error != nil {
			t.Fatal(err)
		}
		if _, ok := addedHashes[r.KeyRemoved.String()]; ok {
			t.Fatal("gc'ed a hash we just added")
		}
	}

	var last cid.Cid
	for a := range out {
		// wait for it to finish
		c, err := cid.Decode(a.(*coreiface.AddEvent).Path.Cid().String())
		if err != nil {
			t.Fatal(err)
		}
		last = c
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	set := cid.NewSet()
	err = dag.EnumerateChildren(ctx, dag.GetLinksWithDAG(node.DAG), last, set.Visit)
	if err != nil {
		t.Fatal(err)
	}
}

func testAddWPosInfo(t *testing.T, rawLeaves bool) {
	r := &repo.Mock{
		C: config.Config{
			Identity: config.Identity{
				PeerID: testPeerID, // required by offline node
			},
		},
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
	}

	api, err := coreapi.New(
		coreapi.Ctx(context.Background()),

		coreapi.Repo(r),
	)
	if err != nil {
		t.Fatal(err)
	}
	// nolint
	node := api.Node()

	bs := &testBlockstore{GCBlockstore: node.Blockstore, expectedPath: filepath.Join(os.TempDir(), "foo.txt"), t: t}
	bserv := blockservice.New(bs, node.Exchange)
	dserv := dag.NewDAGService(bserv)
	adder, err := coreunix.NewAdder(context.Background(), node.Pinning, bs, dserv)
	if err != nil {
		t.Fatal(err)
	}
	out := make(chan interface{})
	adder.Out = out
	adder.Progress = true
	adder.RawLeaves = rawLeaves
	adder.NoCopy = true

	data := make([]byte, 5*1024*1024)
	rand.New(rand.NewSource(2)).Read(data) // Rand.Read never returns an error
	fileData := ioutil.NopCloser(bytes.NewBuffer(data))
	fileInfo := dummyFileInfo{"foo.txt", int64(len(data)), time.Now()}
	file, _ := files.NewReaderPathFile(filepath.Join(os.TempDir(), "foo.txt"), fileData, &fileInfo)

	go func() {
		defer close(adder.Out)
		_, err = adder.AddAllAndPin(file)
		if err != nil {
			t.Error(err)
		}
	}()
	for range out {
	}

	exp := 0
	nonOffZero := 0
	if rawLeaves {
		exp = 1
		nonOffZero = 19
	}
	if bs.countAtOffsetZero != exp {
		t.Fatalf("expected %d blocks with an offset at zero (one root and one leaf), got %d", exp, bs.countAtOffsetZero)
	}
	if bs.countAtOffsetNonZero != nonOffZero {
		// note: the exact number will depend on the size and the sharding algo. used
		t.Fatalf("expected %d blocks with an offset > 0, got %d", nonOffZero, bs.countAtOffsetNonZero)
	}
}

func TestAddWPosInfo(t *testing.T) {
	testAddWPosInfo(t, false)
}

func TestAddWPosInfoAndRawLeafs(t *testing.T) {
	testAddWPosInfo(t, true)
}

type testBlockstore struct {
	blockstore.GCBlockstore
	expectedPath         string
	t                    *testing.T
	countAtOffsetZero    int
	countAtOffsetNonZero int
}

func (bs *testBlockstore) Put(block blocks.Block) error {
	bs.CheckForPosInfo(block)
	return bs.GCBlockstore.Put(block)
}

func (bs *testBlockstore) PutMany(blocks []blocks.Block) error {
	for _, blk := range blocks {
		bs.CheckForPosInfo(blk)
	}
	return bs.GCBlockstore.PutMany(blocks)
}

func (bs *testBlockstore) CheckForPosInfo(block blocks.Block) {
	fsn, ok := block.(*pi.FilestoreNode)
	if ok {
		posInfo := fsn.PosInfo
		if posInfo.FullPath != bs.expectedPath {
			bs.t.Fatal("PosInfo does not have the expected path")
		}
		if posInfo.Offset == 0 {
			bs.countAtOffsetZero += 1
		} else {
			bs.countAtOffsetNonZero += 1
		}
	}
}

type dummyFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (fi *dummyFileInfo) Name() string       { return fi.name }
func (fi *dummyFileInfo) Size() int64        { return fi.size }
func (fi *dummyFileInfo) Mode() os.FileMode  { return 0 }
func (fi *dummyFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *dummyFileInfo) IsDir() bool        { return false }
func (fi *dummyFileInfo) Sys() interface{}   { return nil }
