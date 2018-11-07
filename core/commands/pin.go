package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	iface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
	pin "github.com/ipfs/go-ipfs/pin"

	offline "gx/ipfs/QmPpnbwgAuvhUkA9jGooR88ZwZtTUHXXvoQNKdjZC6nYku/go-ipfs-exchange-offline"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	bserv "gx/ipfs/QmVPeMNK9DfGLXDZzs2W4RoFWC9Zq1EnLGmLXtYtWrNdcW/go-blockservice"
	"gx/ipfs/QmYMQuypUbgsdNHmuCBSUJV6wdQVsBHRivNAp3efHJwZJD/go-verifcid"
	dag "gx/ipfs/QmaDBne4KeY3UepeqSVKYpSmQGa3q9zP6x3LfVF2UjF3Hc/go-merkledag"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var PinCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
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
	Pins     []string
	Progress int `json:",omitempty"`
}

const (
	pinForceOptionName     = "force"
	pinRecursiveOptionName = "recursive"
	pinProgressOptionName  = "progress"
)

var addPinCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Pin objects to local storage.",
		ShortDescription: "Stores an IPFS object(s) from a given path locally to disk.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, true, "Path to object(s) to be pinned.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(pinRecursiveOptionName, "r", "Recursively pin the object linked to by the specified object(s).").WithDefault(true),
		cmdkit.BoolOption(pinProgressOptionName, "Show progress"),
	},
	Type: AddPinOutput{},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		defer n.Blockstore.PinLock().Unlock()

		// set recursive flag
		recursive, _, err := req.Option(pinRecursiveOptionName).Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		showProgress, _, _ := req.Option(pinProgressOptionName).Bool()

		if !showProgress {
			added, err := corerepo.Pin(n, api, req.Context(), req.Arguments(), recursive)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
			res.SetOutput(&AddPinOutput{Pins: cidsToStrings(added)})
			return
		}

		out := make(chan interface{})
		res.SetOutput((<-chan interface{})(out))
		v := new(dag.ProgressTracker)
		ctx := v.DeriveContext(req.Context())

		type pinResult struct {
			pins []cid.Cid
			err  error
		}
		ch := make(chan pinResult, 1)
		go func() {
			added, err := corerepo.Pin(n, api, ctx, req.Arguments(), recursive)
			ch <- pinResult{pins: added, err: err}
		}()

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		defer close(out)
		for {
			select {
			case val := <-ch:
				if val.err != nil {
					res.SetError(val.err, cmdkit.ErrNormal)
					return
				}

				if pv := v.Value(); pv != 0 {
					out <- &AddPinOutput{Progress: v.Value()}
				}
				out <- &AddPinOutput{Pins: cidsToStrings(val.pins)}
				return
			case <-ticker.C:
				out <- &AddPinOutput{Progress: v.Value()}
			case <-ctx.Done():
				log.Error(ctx.Err())
				res.SetError(ctx.Err(), cmdkit.ErrNormal)
				return
			}
		}
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			var added []string

			switch out := v.(type) {
			case *AddPinOutput:
				if out.Pins != nil {
					added = out.Pins
				} else {
					// this can only happen if the progress option is set
					fmt.Fprintf(res.Stderr(), "Fetched/Processed %d nodes\r", out.Progress)
				}

				if res.Error() != nil {
					return nil, res.Error()
				}
			default:
				return nil, e.TypeErr(out, v)
			}

			var pintype string
			rec, found, _ := res.Request().Option("recursive").Bool()
			if rec || !found {
				pintype = "recursively"
			} else {
				pintype = "directly"
			}

			buf := new(bytes.Buffer)
			for _, k := range added {
				fmt.Fprintf(buf, "pinned %s %s\n", k, pintype)
			}
			return buf, nil
		},
	},
}

var rmPinCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Remove pinned objects from local storage.",
		ShortDescription: `
Removes the pin from the given object allowing it to be garbage
collected if needed. (By default, recursively. Use -r=false for direct pins.)
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, true, "Path to object(s) to be unpinned.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(pinForceOptionName, "f", "Ignore nonexistent pins"),
		cmdkit.BoolOption(pinRecursiveOptionName, "r", "Recursively unpin the object linked to by the specified object(s).").WithDefault(true),
	},
	Type: PinOutput{},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		// set recursive flag
		recursive, _, err := req.Option(pinRecursiveOptionName).Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		force, _, err := req.Option(pinForceOptionName).Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		removed, err := corerepo.Unpin(n, api, req.Context(), req.Arguments(), recursive)
		if force && err == pin.ErrNotPinned {
			res.SetOutput(nil)
			return
		}

		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&PinOutput{cidsToStrings(removed)})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			added, ok := v.(*PinOutput)
			if !ok {
				return nil, e.TypeErr(added, v)
			}

			buf := new(bytes.Buffer)
			for _, k := range added.Pins {
				fmt.Fprintf(buf, "unpinned %s\n", k)
			}
			return buf, nil
		},
	},
}

const (
	pinTypeOptionName  = "type"
	pinQuietOptionName = "quiet"
)

var listPinCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
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

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", false, true, "Path to object(s) to be listed."),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(pinTypeOptionName, "t", "The type of pinned keys to list. Can be \"direct\", \"indirect\", \"recursive\", or \"all\".").WithDefault("all"),
		cmdkit.BoolOption(pinQuietOptionName, "q", "Write just hashes of objects."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		typeStr, _, err := req.Option(pinTypeOptionName).String()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		switch typeStr {
		case "all", "direct", "indirect", "recursive":
		default:
			err = fmt.Errorf("invalid type '%s', must be one of {direct, indirect, recursive, all}", typeStr)
			res.SetError(err, cmdkit.ErrClient)
			return
		}

		var keys map[string]RefKeyObject

		if len(req.Arguments()) > 0 {
			keys, err = pinLsKeys(req.Context(), req.Arguments(), typeStr, n, api)
		} else {
			keys, err = pinLsAll(req.Context(), typeStr, n)
		}

		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
		} else {
			res.SetOutput(&RefKeyList{Keys: keys})
		}
	},
	Type: RefKeyList{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			quiet, _, err := res.Request().Option(pinQuietOptionName).Bool()
			if err != nil {
				return nil, err
			}

			keys, ok := v.(*RefKeyList)
			if !ok {
				return nil, e.TypeErr(keys, v)
			}
			out := new(bytes.Buffer)
			for k, v := range keys.Keys {
				if quiet {
					fmt.Fprintf(out, "%s\n", k)
				} else {
					fmt.Fprintf(out, "%s %s\n", k, v.Type)
				}
			}
			return out, nil
		},
	},
}

const (
	pinUnpinOptionName = "unpin"
)

var updatePinCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Update a recursive pin",
		ShortDescription: `
Updates one pin to another, making sure that all objects in the new pin are
local.  Then removes the old pin. This is an optimized version of adding the
new pin and removing the old one.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("from-path", true, false, "Path to old object."),
		cmdkit.StringArg("to-path", true, false, "Path to new object to be pinned."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(pinUnpinOptionName, "Remove the old pin.").WithDefault(true),
	},
	Type: PinOutput{},
	Run: func(req cmds.Request, res cmds.Response) {
		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		unpin, _, err := req.Option(pinUnpinOptionName).Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		from, err := iface.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		to, err := iface.ParsePath(req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = api.Pin().Update(req.Context(), from, to, options.Pin.Unpin(unpin))
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&PinOutput{Pins: []string{from.String(), to.String()}})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}
			added, ok := v.(*PinOutput)
			if !ok {
				return nil, e.TypeErr(added, v)
			}

			buf := new(bytes.Buffer)
			fmt.Fprintf(buf, "updated %s to %s\n", added.Pins[0], added.Pins[1])
			return buf, nil
		},
	},
}

const (
	pinVerboseOptionName = "verbose"
)

var verifyPinCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Verify that recursive pins are complete.",
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(pinVerboseOptionName, "Also write the hashes of non-broken pins."),
		cmdkit.BoolOption(pinQuietOptionName, "q", "Write just hashes of broken pins."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		verbose, _, _ := res.Request().Option(pinVerboseOptionName).Bool()
		quiet, _, _ := res.Request().Option(pinQuietOptionName).Bool()

		if verbose && quiet {
			res.SetError(fmt.Errorf("the --verbose and --quiet options can not be used at the same time"), cmdkit.ErrNormal)
		}

		opts := pinVerifyOpts{
			explain:   !quiet,
			includeOk: verbose,
		}
		out := pinVerify(req.Context(), n, opts)

		res.SetOutput(out)
	},
	Type: PinVerifyRes{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			quiet, _, _ := res.Request().Option(pinQuietOptionName).Bool()

			out, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}
			r, ok := out.(*PinVerifyRes)
			if !ok {
				return nil, e.TypeErr(r, out)
			}

			buf := &bytes.Buffer{}
			if quiet && !r.Ok {
				fmt.Fprintf(buf, "%s\n", r.Cid)
			} else if !quiet {
				r.Format(buf)
			}

			return buf, nil
		},
	},
}

type RefKeyObject struct {
	Type string
}

type RefKeyList struct {
	Keys map[string]RefKeyObject
}

func pinLsKeys(ctx context.Context, args []string, typeStr string, n *core.IpfsNode, api iface.CoreAPI) (map[string]RefKeyObject, error) {

	mode, ok := pin.StringToMode(typeStr)
	if !ok {
		return nil, fmt.Errorf("invalid pin mode '%s'", typeStr)
	}

	keys := make(map[string]RefKeyObject)

	for _, p := range args {
		pth, err := iface.ParsePath(p)
		if err != nil {
			return nil, err
		}

		c, err := api.ResolvePath(ctx, pth)
		if err != nil {
			return nil, err
		}

		pinType, pinned, err := n.Pinning.IsPinnedWithType(c.Cid(), mode)
		if err != nil {
			return nil, err
		}

		if !pinned {
			return nil, fmt.Errorf("path '%s' is not pinned", p)
		}

		switch pinType {
		case "direct", "indirect", "recursive", "internal":
		default:
			pinType = "indirect through " + pinType
		}
		keys[c.Cid().String()] = RefKeyObject{
			Type: pinType,
		}
	}

	return keys, nil
}

func pinLsAll(ctx context.Context, typeStr string, n *core.IpfsNode) (map[string]RefKeyObject, error) {

	keys := make(map[string]RefKeyObject)

	AddToResultKeys := func(keyList []cid.Cid, typeStr string) {
		for _, c := range keyList {
			keys[c.String()] = RefKeyObject{
				Type: typeStr,
			}
		}
	}

	if typeStr == "direct" || typeStr == "all" {
		AddToResultKeys(n.Pinning.DirectKeys(), "direct")
	}
	if typeStr == "indirect" || typeStr == "all" {
		set := cid.NewSet()
		for _, k := range n.Pinning.RecursiveKeys() {
			err := dag.EnumerateChildren(ctx, dag.GetLinksWithDAG(n.DAG), k, set.Visit)
			if err != nil {
				return nil, err
			}
		}
		AddToResultKeys(set.Keys(), "indirect")
	}
	if typeStr == "recursive" || typeStr == "all" {
		AddToResultKeys(n.Pinning.RecursiveKeys(), "recursive")
	}

	return keys, nil
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

func pinVerify(ctx context.Context, n *core.IpfsNode, opts pinVerifyOpts) <-chan interface{} {
	visited := make(map[string]PinStatus)

	bs := n.Blocks.Blockstore()
	DAG := dag.NewDAGService(bserv.New(bs, offline.Exchange(bs)))
	getLinks := dag.GetLinksWithDAG(DAG)
	recPins := n.Pinning.RecursiveKeys()

	var checkPin func(root cid.Cid) PinStatus
	checkPin = func(root cid.Cid) PinStatus {
		key := root.String()
		if status, ok := visited[key]; ok {
			return status
		}

		if err := verifcid.ValidateCid(root); err != nil {
			status := PinStatus{Ok: false}
			if opts.explain {
				status.BadNodes = []BadNode{BadNode{Cid: key, Err: err.Error()}}
			}
			visited[key] = status
			return status
		}

		links, err := getLinks(ctx, root)
		if err != nil {
			status := PinStatus{Ok: false}
			if opts.explain {
				status.BadNodes = []BadNode{BadNode{Cid: key, Err: err.Error()}}
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
				case out <- &PinVerifyRes{cid.String(), pinStatus}:
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

func cidsToStrings(cs []cid.Cid) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.String())
	}
	return out
}
