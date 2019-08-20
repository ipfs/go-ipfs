package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	core "github.com/ipfs/go-ipfs/core"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	pin "github.com/ipfs/go-ipfs/pin"

	bserv "github.com/ipfs/go-blockservice"
	cid "github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmds "github.com/ipfs/go-ipfs-cmds"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	dag "github.com/ipfs/go-merkledag"
	verifcid "github.com/ipfs/go-verifcid"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

var PinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Pin (and unpin) objects to local storage.",
	},

	Subcommands: map[string]*cmds.Command{
		"add":    addPinCmd,
		"rm":     rmPinCmd,
		"ls":     listPinCmd,
		"verify": verifyPinCmd,
		"update": updatePinCmd,
	},
}

type PinOutput struct {
	Pins []string
}

type AddPinOutput struct {
	Pins        []string
	OngoingPins []string `json:",omitempty"`
	Progress    int      `json:",omitempty"`
}

const (
	pinRecursiveOptionName       = "recursive"
	pinProgressOptionName        = "progress"
	pinProgressVerboseOptionName = "verbose"
)

var addPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Pin objects to local storage.",
		ShortDescription: "Stores an IPFS object(s) from a given path locally to disk.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to object(s) to be pinned.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(pinRecursiveOptionName, "r", "Recursively pin the object linked to by the specified object(s).").WithDefault(true),
		cmds.BoolOption(pinProgressOptionName, "Show progress"),
		cmds.BoolOption(pinProgressVerboseOptionName, "Show progress more detailed"),
	},
	Type: AddPinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		// set recursive flag
		recursive, _ := req.Options[pinRecursiveOptionName].(bool)
		showProgressBasic, _ := req.Options[pinProgressOptionName].(bool)
		showProgressVerbose, _ := req.Options[pinProgressVerboseOptionName].(bool)
		showProgress := showProgressBasic || showProgressVerbose

		if err := req.ParseBodyArgs(); err != nil {
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		if !showProgress {
			added, err := pinAddMany(req.Context, api, enc, req.Arguments, recursive)
			if err != nil {
				return err
			}

			return cmds.EmitOnce(res, &AddPinOutput{Pins: added})
		}

		progressTracker := dag.NewProgressTracker()
		ctx := dag.WithProgressTracker(req.Context, progressTracker)

		type pinResult struct {
			pins []string
			err  error
		}

		ch := make(chan pinResult, 1)
		go func() {
			added, err := pinAddMany(ctx, api, enc, req.Arguments, recursive)
			ch <- pinResult{pins: added, err: err}
		}()

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		progressOutput := func() *AddPinOutput {
			cids := progressTracker.PopPlannedToPin()
			ongoingPins := make([]string, 0, len(cids))
			for _, cid := range cids {
				ongoingPins = append(ongoingPins, cid.String())
			}

			return &AddPinOutput{
				Progress:    progressTracker.TotalToPin(),
				OngoingPins: ongoingPins,
			}
		}

		for {
			select {
			case val := <-ch:
				if val.err != nil {
					return val.err
				}

				if pv := progressTracker.TotalToPin(); pv != 0 {
					if err := res.Emit(progressOutput()); err != nil {
						return err
					}
				}

				return res.Emit(&AddPinOutput{Pins: val.pins})
			case <-ticker.C:
				err := res.Emit(progressOutput())
				if err != nil {
					return err
				}
			case <-ctx.Done():
				log.Error(ctx.Err())
				return ctx.Err()
			}
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *AddPinOutput) error {
			rec, found := req.Options["recursive"].(bool)
			var pintype string
			if rec || !found {
				pintype = "recursively"
			} else {
				pintype = "directly"
			}

			for _, k := range out.Pins {
				fmt.Fprintf(w, "pinned %s %s\n", k, pintype)
			}

			return nil
		}),
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			showDigitProgress, _ := res.Request().Options[pinProgressOptionName].(bool)
			showVerboseProgress, _ := res.Request().Options[pinProgressVerboseOptionName].(bool)

			for {
				v, err := res.Next()
				if err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}

				out, ok := v.(*AddPinOutput)
				if !ok {
					return e.TypeErr(out, v)
				}

				// this can only happen if the progress/verbose options are set
				if inProgress := out.Pins == nil; inProgress {
					if showVerboseProgress {
						for _, pin := range out.OngoingPins {
							fmt.Fprintf(os.Stderr,
								"Pinning %s\n", pin)
						}
					} else if showDigitProgress {
						fmt.Fprintf(os.Stderr,
							"Fetched/Processed %d nodes\r", out.Progress)
					}
				} else {
					err = re.Emit(out)
					if err != nil {
						return err
					}
				}
			}
		},
	},
}

func pinAddMany(ctx context.Context, api coreiface.CoreAPI, enc cidenc.Encoder, paths []string, recursive bool) ([]string, error) {
	added := make([]string, len(paths))
	for i, b := range paths {
		rp, err := api.ResolvePath(ctx, path.New(b))
		if err != nil {
			return nil, err
		}

		if err := api.Pin().Add(ctx, rp, options.Pin.Recursive(recursive)); err != nil {
			return nil, err
		}
		added[i] = enc.Encode(rp.Cid())
	}

	return added, nil
}

var rmPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove pinned objects from local storage.",
		ShortDescription: `
Removes the pin from the given object allowing it to be garbage
collected if needed. (By default, recursively. Use -r=false for direct pins.)
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to object(s) to be unpinned.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(pinRecursiveOptionName, "r", "Recursively unpin the object linked to by the specified object(s).").WithDefault(true),
	},
	Type: PinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		// set recursive flag
		recursive, _ := req.Options[pinRecursiveOptionName].(bool)

		if err := req.ParseBodyArgs(); err != nil {
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		pins := make([]string, 0, len(req.Arguments))
		for _, b := range req.Arguments {
			rp, err := api.ResolvePath(req.Context, path.New(b))
			if err != nil {
				return err
			}

			id := enc.Encode(rp.Cid())
			pins = append(pins, id)
			if err := api.Pin().Rm(req.Context, rp, options.Pin.RmRecursive(recursive)); err != nil {
				return err
			}
		}

		return cmds.EmitOnce(res, &PinOutput{pins})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *PinOutput) error {
			for _, k := range out.Pins {
				fmt.Fprintf(w, "unpinned %s\n", k)
			}

			return nil
		}),
	},
}

const (
	pinTypeOptionName   = "type"
	pinQuietOptionName  = "quiet"
	pinStreamOptionName = "stream"
)

var listPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects pinned to local storage.",
		ShortDescription: `
Returns a list of objects that are pinned locally.
By default, all pinned objects are returned, but the '--type' flag or
arguments can restrict that to a specific pin type or to some specific objects
respectively.
`,
		LongDescription: `
Returns a list of objects that are pinned locally.
By default, all pinned objects are returned, but the '--type' flag or
arguments can restrict that to a specific pin type or to some specific objects
respectively.

Use --type=<type> to specify the type of pinned keys to list.
Valid values are:
    * "direct": pin that specific object.
    * "recursive": pin that specific object, and indirectly pin all its
	descendants
    * "indirect": pinned indirectly by an ancestor (like a refcount)
    * "all"

With arguments, the command fails if any of the arguments is not a pinned
object. And if --type=<type> is additionally used, the command will also fail
if any of the arguments is not of the specified type.

Example:
	$ echo "hello" | ipfs add -q
	QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	$ ipfs pin ls
	QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN recursive
	# now remove the pin, and repin it directly
	$ ipfs pin rm QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	unpinned QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	$ ipfs pin add -r=false QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	pinned QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN directly
	$ ipfs pin ls --type=direct
	QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN direct
	$ ipfs pin ls QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN direct
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", false, true, "Path to object(s) to be listed."),
	},
	Options: []cmds.Option{
		cmds.StringOption(pinTypeOptionName, "t", "The type of pinned keys to list. Can be \"direct\", \"indirect\", \"recursive\", or \"all\".").WithDefault("all"),
		cmds.BoolOption(pinQuietOptionName, "q", "Write just hashes of objects."),
		cmds.BoolOption(pinStreamOptionName, "s", "Enable streaming of pins as they are discovered."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		typeStr, _ := req.Options[pinTypeOptionName].(string)
		stream, _ := req.Options[pinStreamOptionName].(bool)

		switch typeStr {
		case "all", "direct", "indirect", "recursive":
		default:
			err = fmt.Errorf("invalid type '%s', must be one of {direct, indirect, recursive, all}", typeStr)
			return err
		}

		// For backward compatibility, we accumulate the pins in the same output type as before.
		emit := res.Emit
		lgcList := map[string]PinLsType{}
		if !stream {
			emit = func(v interface{}) error {
				obj := v.(*PinLsOutputWrapper)
				lgcList[obj.PinLsObject.Cid] = PinLsType{Type: obj.PinLsObject.Type}
				return nil
			}
		}

		if len(req.Arguments) > 0 {
			err = pinLsKeys(req, typeStr, n, api, emit)
		} else {
			err = pinLsAll(req, typeStr, n, emit)
		}
		if err != nil {
			return err
		}

		if !stream {
			return cmds.EmitOnce(res, &PinLsOutputWrapper{
				PinLsList: PinLsList{Keys: lgcList},
			})
		}

		return nil
	},
	Type: &PinLsOutputWrapper{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *PinLsOutputWrapper) error {
			quiet, _ := req.Options[pinQuietOptionName].(bool)
			stream, _ := req.Options[pinStreamOptionName].(bool)

			if stream {
				if quiet {
					fmt.Fprintf(w, "%s\n", out.PinLsObject.Cid)
				} else {
					fmt.Fprintf(w, "%s %s\n", out.PinLsObject.Cid, out.PinLsObject.Type)
				}
				return nil
			}

			for k, v := range out.PinLsList.Keys {
				if quiet {
					fmt.Fprintf(w, "%s\n", k)
				} else {
					fmt.Fprintf(w, "%s %s\n", k, v.Type)
				}
			}

			return nil
		}),
	},
}

// PinLsOutputWrapper is the output type of the pin ls command.
// Pin ls needs to output two different type depending on if it's streamed or not.
// We use this to bypass the cmds lib refusing to have interface{}
type PinLsOutputWrapper struct {
	PinLsList
	PinLsObject
}

// PinLsList is a set of pins with their type
type PinLsList struct {
	Keys map[string]PinLsType `json:",omitempty"`
}

// PinLsType contains the type of a pin
type PinLsType struct {
	Type string
}

// PinLsObject contains the description of a pin
type PinLsObject struct {
	Cid  string `json:",omitempty"`
	Type string `json:",omitempty"`
}

func pinLsKeys(req *cmds.Request, typeStr string, n *core.IpfsNode, api coreiface.CoreAPI, emit func(value interface{}) error) error {
	mode, ok := pin.StringToMode(typeStr)
	if !ok {
		return fmt.Errorf("invalid pin mode '%s'", typeStr)
	}

	enc, err := cmdenv.GetCidEncoder(req)
	if err != nil {
		return err
	}

	for _, p := range req.Arguments {
		c, err := api.ResolvePath(req.Context, path.New(p))
		if err != nil {
			return err
		}

		pinType, pinned, err := n.Pinning.IsPinnedWithType(c.Cid(), mode)
		if err != nil {
			return err
		}

		if !pinned {
			return fmt.Errorf("path '%s' is not pinned", p)
		}

		switch pinType {
		case "direct", "indirect", "recursive", "internal":
		default:
			pinType = "indirect through " + pinType
		}

		err = emit(&PinLsOutputWrapper{
			PinLsObject: PinLsObject{
				Type: pinType,
				Cid:  enc.Encode(c.Cid()),
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func pinLsAll(req *cmds.Request, typeStr string, n *core.IpfsNode, emit func(value interface{}) error) error {
	enc, err := cmdenv.GetCidEncoder(req)
	if err != nil {
		return err
	}

	keys := cid.NewSet()

	AddToResultKeys := func(keyList []cid.Cid, typeStr string) error {
		for _, c := range keyList {
			if keys.Visit(c) {
				err := emit(&PinLsOutputWrapper{
					PinLsObject: PinLsObject{
						Type: typeStr,
						Cid:  enc.Encode(c),
					},
				})
				if err != nil {
					return err
				}
			}
		}
		return nil
	}

	if typeStr == "direct" || typeStr == "all" {
		err := AddToResultKeys(n.Pinning.DirectKeys(), "direct")
		if err != nil {
			return err
		}
	}
	if typeStr == "recursive" || typeStr == "all" {
		err := AddToResultKeys(n.Pinning.RecursiveKeys(), "recursive")
		if err != nil {
			return err
		}
	}
	if typeStr == "indirect" || typeStr == "all" {
		for _, k := range n.Pinning.RecursiveKeys() {
			var visitErr error
			err := dag.Walk(req.Context, dag.GetLinksWithDAG(n.DAG), k, func(c cid.Cid) bool {
				r := keys.Visit(c)
				if r {
					err := emit(&PinLsOutputWrapper{
						PinLsObject: PinLsObject{
							Type: "indirect",
							Cid:  enc.Encode(c),
						},
					})
					if err != nil {
						visitErr = err
					}
				}
				return r
			}, dag.SkipRoot(), dag.Concurrent())

			if visitErr != nil {
				return visitErr
			}
			if err != nil {
				return err
			}
		}
	}

	return nil
}

const (
	pinUnpinOptionName = "unpin"
)

var updatePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Update a recursive pin",
		ShortDescription: `
Efficiently pins a new object based on differences from an existing one and,
by default, removes the old pin.

This commands is useful when the new pin contains many similarities or is a
derivate of an existing one, particuarly for large objects. This allows a more
efficient DAG-traversal which fully skips already-pinned branches from the old
object. As a requirement, the old object needs to be an existing recursive
pin.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("from-path", true, false, "Path to old object."),
		cmds.StringArg("to-path", true, false, "Path to a new object to be pinned."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(pinUnpinOptionName, "Remove the old pin.").WithDefault(true),
	},
	Type: PinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		unpin, _ := req.Options[pinUnpinOptionName].(bool)

		// Resolve the paths ahead of time so we can return the actual CIDs
		from, err := api.ResolvePath(req.Context, path.New(req.Arguments[0]))
		if err != nil {
			return err
		}
		to, err := api.ResolvePath(req.Context, path.New(req.Arguments[1]))
		if err != nil {
			return err
		}

		err = api.Pin().Update(req.Context, from, to, options.Pin.Unpin(unpin))
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &PinOutput{Pins: []string{enc.Encode(from.Cid()), enc.Encode(to.Cid())}})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *PinOutput) error {
			fmt.Fprintf(w, "updated %s to %s\n", out.Pins[0], out.Pins[1])
			return nil
		}),
	},
}

const (
	pinVerboseOptionName = "verbose"
)

var verifyPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Verify that recursive pins are complete.",
	},
	Options: []cmds.Option{
		cmds.BoolOption(pinVerboseOptionName, "Also write the hashes of non-broken pins."),
		cmds.BoolOption(pinQuietOptionName, "q", "Write just hashes of broken pins."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		verbose, _ := req.Options[pinVerboseOptionName].(bool)
		quiet, _ := req.Options[pinQuietOptionName].(bool)

		if verbose && quiet {
			return fmt.Errorf("the --verbose and --quiet options can not be used at the same time")
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		opts := pinVerifyOpts{
			explain:   !quiet,
			includeOk: verbose,
		}
		out := pinVerify(req.Context, n, opts, enc)

		return res.Emit(out)
	},
	Type: PinVerifyRes{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *PinVerifyRes) error {
			quiet, _ := req.Options[pinQuietOptionName].(bool)

			if quiet && !out.Ok {
				fmt.Fprintf(w, "%s\n", out.Cid)
			} else if !quiet {
				out.Format(w)
			}

			return nil
		}),
	},
}

// PinVerifyRes is the result returned for each pin checked in "pin verify"
type PinVerifyRes struct {
	Cid string
	PinStatus
}

// PinStatus is part of PinVerifyRes, do not use directly
type PinStatus struct {
	Ok       bool
	BadNodes []BadNode `json:",omitempty"`
}

// BadNode is used in PinVerifyRes
type BadNode struct {
	Cid string
	Err string
}

type pinVerifyOpts struct {
	explain   bool
	includeOk bool
}

func pinVerify(ctx context.Context, n *core.IpfsNode, opts pinVerifyOpts, enc cidenc.Encoder) <-chan interface{} {
	visited := make(map[cid.Cid]PinStatus)

	bs := n.Blocks.Blockstore()
	DAG := dag.NewDAGService(bserv.New(bs, offline.Exchange(bs)))
	getLinks := dag.GetLinksWithDAG(DAG)
	recPins := n.Pinning.RecursiveKeys()

	var checkPin func(root cid.Cid) PinStatus
	checkPin = func(root cid.Cid) PinStatus {
		key := root
		if status, ok := visited[key]; ok {
			return status
		}

		if err := verifcid.ValidateCid(root); err != nil {
			status := PinStatus{Ok: false}
			if opts.explain {
				status.BadNodes = []BadNode{BadNode{Cid: enc.Encode(key), Err: err.Error()}}
			}
			visited[key] = status
			return status
		}

		links, err := getLinks(ctx, root)
		if err != nil {
			status := PinStatus{Ok: false}
			if opts.explain {
				status.BadNodes = []BadNode{BadNode{Cid: enc.Encode(key), Err: err.Error()}}
			}
			visited[key] = status
			return status
		}

		status := PinStatus{Ok: true}
		for _, lnk := range links {
			res := checkPin(lnk.Cid)
			if !res.Ok {
				status.Ok = false
				status.BadNodes = append(status.BadNodes, res.BadNodes...)
			}
		}

		visited[key] = status
		return status
	}

	out := make(chan interface{})
	go func() {
		defer close(out)
		for _, cid := range recPins {
			pinStatus := checkPin(cid)
			if !pinStatus.Ok || opts.includeOk {
				select {
				case out <- &PinVerifyRes{enc.Encode(cid), pinStatus}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out
}

// Format formats PinVerifyRes
func (r PinVerifyRes) Format(out io.Writer) {
	if r.Ok {
		fmt.Fprintf(out, "%s ok\n", r.Cid)
	} else {
		fmt.Fprintf(out, "%s broken\n", r.Cid)
		for _, e := range r.BadNodes {
			fmt.Fprintf(out, "  %s: %s\n", e.Cid, e.Err)
		}
	}
}
