package io

import (
	"github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
	"github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("dagwriter")

type DagWriter struct {
	dagserv   dag.DAGService
	node      *dag.Node
	totalSize int64
	splChan   chan []byte
	done      chan struct{}
	splitter  chunk.BlockSplitter
	seterr    error
}

func NewDagWriter(ds dag.DAGService, splitter chunk.BlockSplitter) *DagWriter {
	dw := new(DagWriter)
	dw.dagserv = ds
	dw.splChan = make(chan []byte, 8)
	dw.splitter = splitter
	dw.done = make(chan struct{})
	go dw.startSplitter()
	return dw
}

// startSplitter manages splitting incoming bytes and
// creating dag nodes from them. Created nodes are stored
// in the DAGService and then released to the GC.
func (dw *DagWriter) startSplitter() {

	// Since the splitter functions take a reader (and should!)
	// we wrap our byte chan input in a reader
	r := util.NewByteChanReader(dw.splChan)
	blkchan := dw.splitter.Split(r)

	// First data block is reserved for storage in the root node
	first := <-blkchan
	mbf := new(ft.MultiBlock)
	root := new(dag.Node)

	for blkData := range blkchan {
		// Store the block size in the root node
		mbf.AddBlockSize(uint64(len(blkData)))
		node := &dag.Node{Data: ft.WrapData(blkData)}
		_, err := dw.dagserv.Add(node)
		if err != nil {
			dw.seterr = err
			log.Critical("Got error adding created node to dagservice: %s", err)
			return
		}

		// Add a link to this node without storing a reference to the memory
		err = root.AddNodeLinkClean("", node)
		if err != nil {
			dw.seterr = err
			log.Critical("Got error adding created node to root node: %s", err)
			return
		}
	}

	// Generate the root node data
	mbf.Data = first
	data, err := mbf.GetBytes()
	if err != nil {
		dw.seterr = err
		log.Critical("Failed generating bytes for multiblock file: %s", err)
		return
	}
	root.Data = data

	// Add root node to the dagservice
	_, err = dw.dagserv.Add(root)
	if err != nil {
		dw.seterr = err
		log.Critical("Got error adding created node to dagservice: %s", err)
		return
	}
	dw.node = root
	dw.done <- struct{}{}
}

func (dw *DagWriter) Write(b []byte) (int, error) {
	if dw.seterr != nil {
		return 0, dw.seterr
	}
	dw.splChan <- b
	return len(b), nil
}

// Close the splitters input channel and wait for it to finish
// Must be called to finish up splitting, otherwise split method
// will never halt
func (dw *DagWriter) Close() error {
	close(dw.splChan)
	<-dw.done
	return nil
}

func (dw *DagWriter) GetNode() *dag.Node {
	return dw.node
}
