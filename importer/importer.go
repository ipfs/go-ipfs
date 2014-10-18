package importer

import (
	"fmt"
	"io"
	"os"

	"github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
	"github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("importer")

// BlockSizeLimit specifies the maximum size an imported block can have.
var BlockSizeLimit = int64(1048576) // 1 MB

// ErrSizeLimitExceeded signals that a block is larger than BlockSizeLimit.
var ErrSizeLimitExceeded = fmt.Errorf("object size limit exceeded")

// todo: incremental construction with an ipfs node. dumping constructed
// objects into the datastore, to avoid buffering all in memory

// NewDagFromReader constructs a Merkle DAG from the given io.Reader.
// size required for block construction.
func NewDagFromReader(r io.Reader) (*dag.Node, error) {
	return NewDagFromReaderWithSplitter(r, chunk.DefaultSplitter)
}

func NewDagFromReaderWithSplitter(r io.Reader, spl chunk.BlockSplitter) (*dag.Node, error) {
	// get rid of the r argument and push the r into spl up in NewDagFromReader
	spl.Push(r)

	first, err := spl.Next()
	if err != nil {
		return nil, err
	}

	root := &dag.Node{}

	mbf := new(ft.MultiBlock)
	for {
		blk, err := spl.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		mbf.AddBlockSize(uint64(len(blk)))
		child := &dag.Node{Data: ft.WrapData(blk)}
		err = root.AddNodeLink("", child)
		if err != nil {
			return nil, err
		}
	}

	mbf.Data = first
	data, err := mbf.GetBytes()
	if err != nil {
		return nil, err
	}

	root.Data = data
	return root, nil
}

// NewDagFromFile constructs a Merkle DAG from the file at given path.
func NewDagFromFile(fpath string) (*dag.Node, error) {
	stat, err := os.Stat(fpath)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("`%s` is a directory", fpath)
	}

	f, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return NewDagFromReader(f)
}
