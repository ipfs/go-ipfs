package dagcmd

import (
	"errors"
	"fmt"
	"io"

	cid "github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	ipld "github.com/ipfs/go-ipld-format"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"

	cmds "github.com/ipfs/go-ipfs-cmds"
	gocar "github.com/ipld/go-car"
)

func dagImport(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {

	node, err := cmdenv.GetNode(env)
	if err != nil {
		return err
	}

	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}

	// on import ensure we do not reach out to the network for any reason
	// if a pin based on what is imported + what is in the blockstore
	// isn't possible: tough luck
	api, err = api.WithOptions(options.Api.Offline(true))
	if err != nil {
		return err
	}

	// grab a pinlock ( which doubles as a GC lock ) so that regardless of the
	// size of the streamed-in cars nothing will disappear on us before we had
	// a chance to roots that may show up at the very end
	// This is especially important for use cases like dagger:
	//    ipfs dag import $( ... | ipfs-dagger --stdout=carfifos )
	//
	unlocker := node.Blockstore.PinLock()
	defer unlocker.Unlock()

	doPinRoots, _ := req.Options[pinRootsOptionName].(bool)

	retCh := make(chan importResult, 1)
	go importWorker(req, res, api, retCh)

	done := <-retCh
	if done.err != nil {
		return done.err
	}

	if err := res.Emit(&CarImportOutput{BlockCount: &done.blockCount}); err != nil {
		return err
	}

	// It is not guaranteed that a root in a header is actually present in the same ( or any )
	// .car file. This is the case in version 1, and ideally in further versions too
	// Accumulate any root CID seen in a header, and supplement its actual node if/when encountered
	// We will attempt a pin *only* at the end in case all car files were well formed
	//
	// The boolean value indicates whether we have encountered the root within the car file's
	roots := done.roots

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

			if block, err := node.Blockstore.Get(c); err != nil {
				ret.PinErrorMsg = err.Error()
			} else if nd, err := ipld.Decode(block); err != nil {
				ret.PinErrorMsg = err.Error()
			} else if err := node.Pinning.Pin(req.Context, nd, true); err != nil {
				ret.PinErrorMsg = err.Error()
			} else if err := node.Pinning.Flush(req.Context); err != nil {
				ret.PinErrorMsg = err.Error()
			}

			if ret.PinErrorMsg != "" {
				failedPins++
			}

			if err := res.Emit(&CarImportOutput{Root: &ret}); err != nil {
				return err
			}
		}

		if failedPins > 0 {
			return fmt.Errorf(
				"unable to pin all roots: %d out of %d failed",
				failedPins,
				len(roots),
			)
		}
	}

	return nil
}

func importWorker(req *cmds.Request, re cmds.ResponseEmitter, api iface.CoreAPI, ret chan importResult) {

	// this is *not* a transaction
	// it is simply a way to relieve pressure on the blockstore
	// similar to pinner.Pin/pinner.Flush
	batch := ipld.NewBatch(req.Context, api.Dag())

	roots := make(map[cid.Cid]struct{})
	var blockCount uint64

	it := req.Files.Entries()
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

				if err := batch.Add(req.Context, nd); err != nil {
					return err
				}
				blockCount++
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

	ret <- importResult{blockCount: blockCount, roots: roots}
}
