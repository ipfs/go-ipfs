package coreapi

import (
	"context"
	"errors"
	"fmt"
	"io"

	cid "github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs-pinner"
	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
	gocar "github.com/ipld/go-car"
)

type dagAPI struct {
	ipld.DAGService

	core *CoreAPI
}

type pinningAdder CoreAPI

func (adder *pinningAdder) Add(ctx context.Context, nd ipld.Node) error {
	defer adder.blockstore.PinLock().Unlock()

	if err := adder.dag.Add(ctx, nd); err != nil {
		return err
	}

	adder.pinning.PinWithMode(nd.Cid(), pin.Recursive)

	return adder.pinning.Flush(ctx)
}

func (adder *pinningAdder) AddMany(ctx context.Context, nds []ipld.Node) error {
	defer adder.blockstore.PinLock().Unlock()

	if err := adder.dag.AddMany(ctx, nds); err != nil {
		return err
	}

	cids := cid.NewSet()

	for _, nd := range nds {
		c := nd.Cid()
		if cids.Visit(c) {
			adder.pinning.PinWithMode(c, pin.Recursive)
		}
	}

	return adder.pinning.Flush(ctx)
}

func (api *dagAPI) Pinning() ipld.NodeAdder {
	return (*pinningAdder)(api.core)
}

func (api *dagAPI) Session(ctx context.Context) ipld.NodeGetter {
	return dag.NewSession(ctx, api.DAGService)
}

// RootMeta is the metadata for a root pinning response
type RootMeta struct {
	Cid         cid.Cid
	PinErrorMsg string
}

// ImportMany imports CAR files into the blockstore and returns the root CIDs. If pinning was requested errors that
// occurred when importing each root will be returned as well.
//
// This function's API is not yet stable
func (api *dagAPI) ImportMany(ctx context.Context, directory files.Directory, doPinRoots bool) (<-chan RootMeta, <-chan error) {
	// grab a pinlock ( which doubles as a GC lock ) so that regardless of the
	// size of the streamed-in cars nothing will disappear on us before we had
	// a chance to roots that may show up at the very end
	// This is especially important for use cases like dagger:
	//    ipfs dag import $( ... | ipfs-dagger --stdout=carfifos )
	//
	unlocker := api.core.blockstore.PinLock()
	defer unlocker.Unlock()

	retCh := make(chan importResult, 1)
	go api.importWorker(ctx, directory, retCh)

	rootsCh := make(chan RootMeta, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)
		defer close(rootsCh)

		done := <-retCh
		if done.err != nil {
			errCh <- done.err
			return
		}

		// It is not guaranteed that a root in a header is actually present in the same ( or any )
		// .car file. This is the case in version 1, and ideally in further versions too
		// Accumulate any root CID seen in a header, and supplement its actual node if/when encountered
		// We will attempt a pin *only* at the end in case all car files were well formed
		//
		// The boolean value indicates whether we have encountered the root within the car file's
		roots := done.roots

		// If we are not pinning roots then just return all the roots
		if !doPinRoots {
			for r := range roots {
				select {
				case rootsCh <- RootMeta{Cid: r}:
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				}
			}
			return
		}

		// opportunistic pinning: try whatever sticks
		if doPinRoots {

			var failedPins int
			for c := range roots {

				// We need to re-retrieve a block, convert it to ipld, and feed it
				// to the Pinning interface, sigh...
				//
				// If we didn't have the problem of inability to take multiple pinlocks,
				// we could use the api directly like so (though internally it does the same):
				//
				// // not ideal, but the pinning api takes only paths :(
				// rp := path.NewResolvedPath(
				// 	ipfspath.FromCid(c),
				// 	c,
				// 	c,
				// 	"",
				// )
				//
				// if err := api.Pin().Add(req.Context, rp, options.Pin.Recursive(true)); err != nil {

				ret := RootMeta{Cid: c}

				if block, err := api.core.blockstore.Get(c); err != nil {
					ret.PinErrorMsg = err.Error()
				} else if nd, err := ipld.Decode(block); err != nil {
					ret.PinErrorMsg = err.Error()
				} else if err := api.core.pinning.Pin(ctx, nd, true); err != nil {
					ret.PinErrorMsg = err.Error()
				} else if err := api.core.pinning.Flush(ctx); err != nil {
					ret.PinErrorMsg = err.Error()
				}

				if ret.PinErrorMsg != "" {
					failedPins++
				}

				select {
				case rootsCh <- ret:
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				}
			}
		}
	}()

	return rootsCh, errCh
}

type importResult struct {
	roots map[cid.Cid]struct{}
	err   error
}

func (api *dagAPI) importWorker(ctx context.Context, dir files.Directory, ret chan importResult) {
	// this is *not* a transaction
	// it is simply a way to relieve pressure on the blockstore
	// similar to pinner.Pin/pinner.Flush
	batch := ipld.NewBatch(ctx, api)

	roots := make(map[cid.Cid]struct{})

	it := dir.Entries()
	for it.Next() {

		file := files.FileFromEntry(it)
		if file == nil {
			ret <- importResult{err: errors.New("expected a file handle")}
			return
		}

		// wrap a defer-closer-scope
		//
		// every single file in it() is already open before we start
		// just close here sooner rather than later for neatness
		// and to surface potential errors writing on closed fifos
		// this won't/can't help with not running out of handles
		err := func() error {
			defer file.Close()

			car, err := gocar.NewCarReader(file)
			if err != nil {
				return err
			}

			// Be explicit here, until the spec is finished
			if car.Header.Version != 1 {
				return errors.New("only car files version 1 supported at present")
			}

			for _, c := range car.Header.Roots {
				roots[c] = struct{}{}
			}

			for {
				block, err := car.Next()
				if err != nil && err != io.EOF {
					return err
				} else if block == nil {
					break
				}

				// the double-decode is suboptimal, but we need it for batching
				nd, err := ipld.Decode(block)
				if err != nil {
					return err
				}

				if err := batch.Add(ctx, nd); err != nil {
					return err
				}
			}

			return nil
		}()

		if err != nil {
			ret <- importResult{err: err}
			return
		}
	}

	if err := it.Err(); err != nil {
		ret <- importResult{err: err}
		return
	}

	if err := batch.Commit(); err != nil {
		ret <- importResult{err: err}
		return
	}

	ret <- importResult{roots: roots}
}

// Export exports the DAG rooted at a given CID into a CAR file. The error channel will return at most one error and
// the reader is usable before the error channel has been closed.
func (api *dagAPI) Export(ctx context.Context, c cid.Cid) (io.ReadCloser, <-chan error) {
	// Code disabled until descent-issue in go-ipld-prime is fixed
	// https://github.com/ribasushi/gip-muddle-up
	//
	// sb := gipselectorbuilder.NewSelectorSpecBuilder(gipfree.NodeBuilder())
	// car := gocar.NewSelectiveCar(
	// 	req.Context,
	// 	<needs to be fixed to take format.NodeGetter as well>,
	// 	[]gocar.Dag{gocar.Dag{
	// 		Root: c,
	// 		Selector: sb.ExploreRecursive(
	// 			gipselector.RecursionLimitNone(),
	// 			sb.ExploreAll(sb.ExploreRecursiveEdge()),
	// 		).Node(),
	// 	}},
	// )
	// ...
	// if err := car.Write(pipeW); err != nil {}

	pipeR, pipeW := io.Pipe()

	errCh := make(chan error, 1) // we only report the 1st error
	go func() {
		defer close(errCh)
		var err error
		if err = gocar.WriteCar(
			ctx,
			api.Session(ctx),
			[]cid.Cid{c},
			pipeW,
		); err != nil {
			errCh <- err
		}

		if closeErr := pipeW.Close(); closeErr != nil && err == nil {
			errCh <- fmt.Errorf("stream flush failed: %s", closeErr)
		}
	}()

	return pipeR, errCh
}

var (
	_ ipld.DAGService  = (*dagAPI)(nil)
	_ dag.SessionMaker = (*dagAPI)(nil)
)
