package dagcmd

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/coredag"
	iface "github.com/ipfs/interface-go-ipfs-core"

	cid "github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	mdag "github.com/ipfs/go-merkledag"
	ipfspath "github.com/ipfs/go-path"
	"github.com/ipfs/interface-go-ipfs-core/options"
	path "github.com/ipfs/interface-go-ipfs-core/path"
	mh "github.com/multiformats/go-multihash"

	gocar "github.com/ipld/go-car"
	//gipfree "github.com/ipld/go-ipld-prime/impl/free"
	//gipselector "github.com/ipld/go-ipld-prime/traversal/selector"
	//gipselectorbuilder "github.com/ipld/go-ipld-prime/traversal/selector/builder"

	"gopkg.in/cheggaaa/pb.v1"
)

const (
	progressOptionName = "progress"
	silentOptionName   = "silent"
	pinRootsOptionName = "pin-roots"
)

var DagCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with ipld dag objects.",
		ShortDescription: `
'ipfs dag' is used for creating and manipulating dag objects/hierarchies.

This subcommand is currently an experimental feature, but it is intended
to deprecate and replace the existing 'ipfs object' command moving forward.
		`,
	},
	Subcommands: map[string]*cmds.Command{
		"put":     DagPutCmd,
		"get":     DagGetCmd,
		"resolve": DagResolveCmd,
		"import":  DagImportCmd,
		"export":  DagExportCmd,
	},
}

// OutputObject is the output type of 'dag put' command
type OutputObject struct {
	Cid cid.Cid
}

// ResolveOutput is the output type of 'dag resolve' command
type ResolveOutput struct {
	Cid     cid.Cid
	RemPath string
}

// CarImportOutput is the output type of the 'dag import' commands
type CarImportOutput struct {
	Root RootMeta
}
type RootMeta struct {
	Cid         cid.Cid
	PinErrorMsg string
}

var DagPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add a dag node to ipfs.",
		ShortDescription: `
'ipfs dag put' accepts input from a file or stdin and parses it
into an object of the specified format.
`,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("object data", true, true, "The object to put").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("format", "f", "Format that the object will be added as.").WithDefault("cbor"),
		cmds.StringOption("input-enc", "Format that the input object will be.").WithDefault("json"),
		cmds.BoolOption("pin", "Pin this object when adding."),
		cmds.StringOption("hash", "Hash function to use").WithDefault(""),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		ienc, _ := req.Options["input-enc"].(string)
		format, _ := req.Options["format"].(string)
		hash, _ := req.Options["hash"].(string)
		dopin, _ := req.Options["pin"].(bool)

		// mhType tells inputParser which hash should be used. MaxUint64 means 'use
		// default hash' (sha256 for cbor, sha1 for git..)
		mhType := uint64(math.MaxUint64)

		if hash != "" {
			var ok bool
			mhType, ok = mh.Names[hash]
			if !ok {
				return fmt.Errorf("%s in not a valid multihash name", hash)
			}
		}

		var adder ipld.NodeAdder = api.Dag()
		if dopin {
			adder = api.Dag().Pinning()
		}
		b := ipld.NewBatch(req.Context, adder)

		it := req.Files.Entries()
		for it.Next() {
			file := files.FileFromEntry(it)
			if file == nil {
				return fmt.Errorf("expected a regular file")
			}
			nds, err := coredag.ParseInputs(ienc, format, file, mhType, -1)
			if err != nil {
				return err
			}
			if len(nds) == 0 {
				return fmt.Errorf("no node returned from ParseInputs")
			}

			for _, nd := range nds {
				err := b.Add(req.Context, nd)
				if err != nil {
					return err
				}
			}

			cid := nds[0].Cid()
			if err := res.Emit(&OutputObject{Cid: cid}); err != nil {
				return err
			}
		}
		if it.Err() != nil {
			return it.Err()
		}

		if err := b.Commit(); err != nil {
			return err
		}

		return nil
	},
	Type: OutputObject{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *OutputObject) error {
			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}
			fmt.Fprintln(w, enc.Encode(out.Cid))
			return nil
		}),
	},
}

var DagGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get a dag node from ipfs.",
		ShortDescription: `
'ipfs dag get' fetches a dag node from ipfs and prints it out in the specified
format.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ref", true, false, "The object to get").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		rp, err := api.ResolvePath(req.Context, path.New(req.Arguments[0]))
		if err != nil {
			return err
		}

		obj, err := api.Dag().Get(req.Context, rp.Cid())
		if err != nil {
			return err
		}

		var out interface{} = obj
		if len(rp.Remainder()) > 0 {
			rem := strings.Split(rp.Remainder(), "/")
			final, _, err := obj.Resolve(rem)
			if err != nil {
				return err
			}
			out = final
		}
		return cmds.EmitOnce(res, &out)
	},
}

// DagResolveCmd returns address of highest block within a path and a path remainder
var DagResolveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Resolve ipld block",
		ShortDescription: `
'ipfs dag resolve' fetches a dag node from ipfs, prints its address and remaining path.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ref", true, false, "The path to resolve").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		rp, err := api.ResolvePath(req.Context, path.New(req.Arguments[0]))
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &ResolveOutput{
			Cid:     rp.Cid(),
			RemPath: rp.Remainder(),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *ResolveOutput) error {
			var (
				enc cidenc.Encoder
				err error
			)
			switch {
			case !cmdenv.CidBaseDefined(req):
				// Not specified, check the path.
				enc, err = cmdenv.CidEncoderFromPath(req.Arguments[0])
				if err == nil {
					break
				}
				// Nope, fallback on the default.
				fallthrough
			default:
				enc, err = cmdenv.GetLowLevelCidEncoder(req)
				if err != nil {
					return err
				}
			}
			p := enc.Encode(out.Cid)
			if out.RemPath != "" {
				p = ipfspath.Join([]string{p, out.RemPath})
			}

			fmt.Fprint(w, p)
			return nil
		}),
	},
	Type: ResolveOutput{},
}

type importResult struct {
	roots map[cid.Cid]struct{}
	err   error
}

var DagImportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Import the contents of .car files",
		ShortDescription: `
'ipfs dag import' imports all blocks present in supplied .car
( Content Address aRchive ) files, recursively pinning any roots
specified in the CAR file headers, unless --pin-roots is set to false.

Note:
  This command will import all blocks in the CAR file, not just those
  reachable from the specified roots. However, these other blocks will
  not be pinned and may be garbage collected later.

  The pinning of the roots happens after all car files are processed,
  permitting import of DAGs spanning multiple files.

  Pinning takes place in offline-mode exclusively, one root at a time.
  If the combination of blocks from the imported CAR files and what is
  currently present in the blockstore does not represent a complete DAG,
  pinning of that individual root will fail.

Maximum supported CAR version: 1
`,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("path", true, true, "The path of a .car file.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(silentOptionName, "No output."),
		cmds.BoolOption(pinRootsOptionName, "Pin optional roots listed in the .car headers after importing.").WithDefault(true),
	},
	Type: CarImportOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {

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

				if block, err := node.Blockstore.Get(req.Context, c); err != nil {
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

				if err := res.Emit(&CarImportOutput{Root: ret}); err != nil {
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
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, event *CarImportOutput) error {

			silent, _ := req.Options[silentOptionName].(bool)
			if silent {
				return nil
			}

			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}

			if event.Root.PinErrorMsg != "" {
				event.Root.PinErrorMsg = fmt.Sprintf("FAILED: %s", event.Root.PinErrorMsg)
			} else {
				event.Root.PinErrorMsg = "success"
			}

			_, err = fmt.Fprintf(
				w,
				"Pinned root\t%s\t%s\n",
				enc.Encode(event.Root.Cid),
				event.Root.PinErrorMsg,
			)
			return err
		}),
	},
}

func importWorker(req *cmds.Request, re cmds.ResponseEmitter, api iface.CoreAPI, ret chan importResult) {

	// this is *not* a transaction
	// it is simply a way to relieve pressure on the blockstore
	// similar to pinner.Pin/pinner.Flush
	batch := ipld.NewBatch(req.Context, api.Dag())

	roots := make(map[cid.Cid]struct{})

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

var DagExportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Streams the selected DAG as a .car stream on stdout.",
		ShortDescription: `
'ipfs dag export' fetches a dag and streams it out as a well-formed .car file.
Note that at present only single root selections / .car files are supported.
The output of blocks happens in strict DAG-traversal, first-seen, order.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "CID of a root to recursively export").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(progressOptionName, "p", "Display progress on CLI. Defaults to true when STDERR is a TTY."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {

		c, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return fmt.Errorf(
				"unable to parse root specification (currently only bare CIDs are supported): %s",
				err,
			)
		}

		node, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

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

		errCh := make(chan error, 2) // we only report the 1st error
		go func() {
			defer func() {
				if err := pipeW.Close(); err != nil {
					errCh <- fmt.Errorf("stream flush failed: %s", err)
				}
				close(errCh)
			}()

			if err := gocar.WriteCar(
				req.Context,
				mdag.NewSession(
					req.Context,
					node.DAG,
				),
				[]cid.Cid{c},
				pipeW,
			); err != nil {
				errCh <- err
			}
		}()

		if err := res.Emit(pipeR); err != nil {
			pipeR.Close() // ignore the error if any
			return err
		}

		err = <-errCh

		// minimal user friendliness
		if err != nil &&
			!node.IsOnline &&
			err == ipld.ErrNotFound {
			err = fmt.Errorf("%s (currently offline, perhaps retry after attaching to the network)", err)
		}

		return err
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {

			var showProgress bool
			val, specified := res.Request().Options[progressOptionName]
			if !specified {
				// default based on TTY availability
				errStat, _ := os.Stderr.Stat()
				if 0 != (errStat.Mode() & os.ModeCharDevice) {
					showProgress = true
				}
			} else if val.(bool) {
				showProgress = true
			}

			// simple passthrough, no progress
			if !showProgress {
				return cmds.Copy(re, res)
			}

			bar := pb.New64(0).SetUnits(pb.U_BYTES)
			bar.Output = os.Stderr
			bar.ShowSpeed = true
			bar.ShowElapsedTime = true
			bar.RefreshRate = 500 * time.Millisecond
			bar.Start()

			var processedOneResponse bool
			for {
				v, err := res.Next()
				if err == io.EOF {

					// We only write the final bar update on success
					// On error it looks too weird
					bar.Finish()

					return re.Close()
				} else if err != nil {
					return re.CloseWithError(err)
				} else if processedOneResponse {
					return re.CloseWithError(errors.New("unexpected multipart response during emit, please file a bugreport"))
				}

				r, ok := v.(io.Reader)
				if !ok {
					// some sort of encoded response, this should not be happening
					return errors.New("unexpected non-stream passed to PostRun: please file a bugreport")
				}

				processedOneResponse = true

				if err := re.Emit(bar.NewProxyReader(r)); err != nil {
					return err
				}
			}
		},
	},
}
