// Package blockstoreutil provides utility functions for Blockstores.
package blockstoreutil

import (
	"fmt"
	"io"

	"github.com/ipfs/go-ipfs/pin"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	ds "gx/ipfs/QmSpg1CvpXQQow5ernt1gNBXaXV6yxyNqi7XoeerWfzB5w/go-datastore"
	bs "gx/ipfs/QmegPGspn3RpTMQ23Fd3GVVMopo1zsEMurudbFMZ5UXBLH/go-ipfs-blockstore"
)

// RemovedBlock is used to respresent the result of removing a block.
// If a block was removed successfully than the Error string will be
// empty.  If a block could not be removed than Error will contain the
// reason the block could not be removed.  If the removal was aborted
// due to a fatal error Hash will be be empty, Error will contain the
// reason, and no more results will be sent.
type RemovedBlock struct {
	Hash  string `json:",omitempty"`
	Error string `json:",omitempty"`
}

// RmBlocksOpts is used to wrap options for RmBlocks().
type RmBlocksOpts struct {
	Prefix string
	Quiet  bool
	Force  bool
}

// RmBlocks removes the blocks provided in the cids slice.
// It returns a channel where objects of type RemovedBlock are placed, when
// not using the Quiet option. Block removal is asynchronous and will
// skip any pinned blocks.
func RmBlocks(blocks bs.GCBlockstore, pins pin.Pinner, cids []cid.Cid, opts RmBlocksOpts) (<-chan interface{}, error) {
	// make the channel large enough to hold any result to avoid
	// blocking while holding the GCLock
	out := make(chan interface{}, len(cids))
	go func() {
		defer close(out)

		unlocker := blocks.GCLock()
		defer unlocker.Unlock()

		stillOkay := FilterPinned(pins, out, cids)

		for _, c := range stillOkay {
			err := blocks.Delete(c.Hash())
			if err != nil && opts.Force && (err == bs.ErrNotFound || err == ds.ErrNotFound) {
				// ignore non-existent blocks
			} else if err != nil {
				out <- &RemovedBlock{Hash: c.String(), Error: err.Error()}
			} else if !opts.Quiet {
				out <- &RemovedBlock{Hash: c.String()}
			}
		}
	}()
	return out, nil
}

// FilterPinned takes a slice of Cids and returns it with the pinned Cids
// removed. If a Cid is pinned, it will place RemovedBlock objects in the given
// out channel, with an error which indicates that the Cid is pinned.
// This function is used in RmBlocks to filter out any blocks which are not
// to be removed (because they are pinned).
func FilterPinned(pins pin.Pinner, out chan<- interface{}, cids []cid.Cid) []cid.Cid {
	stillOkay := make([]cid.Cid, 0, len(cids))
	res, err := pins.CheckIfPinned(cids...)
	if err != nil {
		out <- &RemovedBlock{Error: fmt.Sprintf("pin check failed: %s", err)}
		return nil
	}
	for _, r := range res {
		if !r.Pinned() {
			stillOkay = append(stillOkay, r.Key)
		} else {
			out <- &RemovedBlock{
				Hash:  r.Key.String(),
				Error: r.String(),
			}
		}
	}
	return stillOkay
}

// ProcRmOutput takes a function which returns a result from RmBlocks or EOF if there is no input.
// It then writes to stdout/stderr according to the RemovedBlock object returned from the function.
func ProcRmOutput(next func() (interface{}, error), sout io.Writer, serr io.Writer) error {
	someFailed := false
	for {
		res, err := next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		r := res.(*RemovedBlock)
		if r.Hash == "" && r.Error != "" {
			return fmt.Errorf("aborted: %s", r.Error)
		} else if r.Error != "" {
			someFailed = true
			fmt.Fprintf(serr, "cannot remove %s: %s\n", r.Hash, r.Error)
		} else {
			fmt.Fprintf(sout, "removed %s\n", r.Hash)
		}
	}
	if someFailed {
		return fmt.Errorf("some blocks not removed")
	}
	return nil
}
