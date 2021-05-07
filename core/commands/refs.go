package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"

	cid "github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmds "github.com/ipfs/go-ipfs-cmds"
	ipld "github.com/ipfs/go-ipld-format"
	merkledag "github.com/ipfs/go-merkledag"
	iface "github.com/ipfs/interface-go-ipfs-core"
	path "github.com/ipfs/interface-go-ipfs-core/path"
)

var refsEncoderMap = cmds.EncoderMap{
	cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *RefWrapper) error {
		if out.Err != "" {
			return fmt.Errorf(out.Err)
		}
		fmt.Fprintln(w, out.Ref)

		return nil
	}),
}

// KeyList is a general type for outputting lists of keys
type KeyList struct {
	Keys []cid.Cid
}

const (
	refsFormatOptionName    = "format"
	refsEdgesOptionName     = "edges"
	refsUniqueOptionName    = "unique"
	refsRecursiveOptionName = "recursive"
	refsMaxDepthOptionName  = "max-depth"
)

// RefsCmd is the `ipfs refs` command
var RefsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List links (references) from an object.",
		ShortDescription: `
Lists the hashes of all the links an IPFS or IPNS object(s) contains,
with the following format:

  <link base58 hash>

NOTE: List all references recursively by using the flag '-r'.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"local": RefsLocalCmd,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to the object(s) to list refs from.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(refsFormatOptionName, "Emit edges with given format. Available tokens: <src> <dst> <linkname>.").WithDefault("<dst>"),
		cmds.BoolOption(refsEdgesOptionName, "e", "Emit edge format: `<from> -> <to>`."),
		cmds.BoolOption(refsUniqueOptionName, "u", "Omit duplicate refs from output."),
		cmds.BoolOption(refsRecursiveOptionName, "r", "Recursively list links of child nodes."),
		cmds.IntOption(refsMaxDepthOptionName, "Only for recursive refs, limits fetch and listing to the given depth").WithDefault(-1),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		err := req.ParseBodyArgs()
		if err != nil {
			return err
		}

		ctx := req.Context
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		unique, _ := req.Options[refsUniqueOptionName].(bool)
		recursive, _ := req.Options[refsRecursiveOptionName].(bool)
		maxDepth, _ := req.Options[refsMaxDepthOptionName].(int)
		edges, _ := req.Options[refsEdgesOptionName].(bool)
		format, _ := req.Options[refsFormatOptionName].(string)

		if !recursive {
			maxDepth = 1 // write only direct refs
		}

		if edges {
			if format != "<dst>" {
				return errors.New("using format argument with edges is not allowed")
			}

			format = "<src> -> <dst>"
		}

		// TODO: use session for resolving as well.
		objs, err := objectsForPaths(ctx, api, req.Arguments)
		if err != nil {
			return err
		}

		rw := RefWriter{
			res:      res,
			DAG:      merkledag.NewSession(ctx, api.Dag()),
			Ctx:      ctx,
			Unique:   unique,
			PrintFmt: format,
			MaxDepth: maxDepth,
		}

		for _, o := range objs {
			if _, err := rw.WriteRefs(o, enc); err != nil {
				if err := res.Emit(&RefWrapper{Err: err.Error()}); err != nil {
					return err
				}
			}
		}

		return nil
	},
	Encoders: refsEncoderMap,
	Type:     RefWrapper{},
}

var RefsLocalCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List all local references.",
		ShortDescription: `
Displays the hashes of all local objects. NOTE: This treats all local objects as "raw blocks" and returns CIDv1-Raw CIDs.
`,
	},

	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		// todo: make async
		allKeys, err := n.Blockstore.AllKeysChan(ctx)
		if err != nil {
			return err
		}

		for k := range allKeys {
			err := res.Emit(&RefWrapper{Ref: k.String()})
			if err != nil {
				return err
			}
		}

		return nil
	},
	Encoders: refsEncoderMap,
	Type:     RefWrapper{},
}

func objectsForPaths(ctx context.Context, n iface.CoreAPI, paths []string) ([]cid.Cid, error) {
	roots := make([]cid.Cid, len(paths))
	for i, sp := range paths {
		o, err := n.ResolvePath(ctx, path.New(sp))
		if err != nil {
			return nil, err
		}
		roots[i] = o.Cid()
	}
	return roots, nil
}

type RefWrapper struct {
	Ref string
	Err string
}

type RefWriter struct {
	res cmds.ResponseEmitter
	DAG ipld.NodeGetter
	Ctx context.Context

	Unique   bool
	MaxDepth int
	PrintFmt string

	seen map[string]int
}

// WriteRefs writes refs of the given object to the underlying writer.
func (rw *RefWriter) WriteRefs(c cid.Cid, enc cidenc.Encoder) (int, error) {
	n, err := rw.DAG.Get(rw.Ctx, c)
	if err != nil {
		return 0, err
	}
	return rw.writeRefsRecursive(n, 0, enc)
}

func (rw *RefWriter) writeRefsRecursive(n ipld.Node, depth int, enc cidenc.Encoder) (int, error) {
	nc := n.Cid()

	var count int
	for i, ng := range ipld.GetDAG(rw.Ctx, rw.DAG, n) {
		lc := n.Links()[i].Cid
		goDeeper, shouldWrite := rw.visit(lc, depth+1) // The children are at depth+1

		// Avoid "Get()" on the node and continue with next Link.
		// We can do this if:
		// - We printed it before (thus it was already seen and
		//   fetched with Get()
		// - AND we must not go deeper.
		// This is an optimization for pruned branches which have been
		// visited before.
		if !shouldWrite && !goDeeper {
			continue
		}

		// We must Get() the node because:
		// - it is new (never written)
		// - OR we need to go deeper.
		// This ensures printed refs are always fetched.
		nd, err := ng.Get(rw.Ctx)
		if err != nil {
			return count, err
		}

		// Write this node if not done before (or !Unique)
		if shouldWrite {
			if err := rw.WriteEdge(nc, lc, n.Links()[i].Name, enc); err != nil {
				return count, err
			}
			count++
		}

		// Keep going deeper. This happens:
		// - On unexplored branches
		// - On branches not explored deep enough
		// Note when !Unique, branches are always considered
		// unexplored and only depth limits apply.
		if goDeeper {
			c, err := rw.writeRefsRecursive(nd, depth+1, enc)
			count += c
			if err != nil {
				return count, err
			}
		}
	}

	return count, nil
}

// visit returns two values:
// - the first boolean is true if we should keep traversing the DAG
// - the second boolean is true if we should print the CID
//
// visit will do branch pruning depending on rw.MaxDepth, previously visited
// cids and whether rw.Unique is set. i.e. rw.Unique = false and
// rw.MaxDepth = -1 disables any pruning. But setting rw.Unique to true will
// prune already visited branches at the cost of keeping as set of visited
// CIDs in memory.
func (rw *RefWriter) visit(c cid.Cid, depth int) (bool, bool) {
	atMaxDepth := rw.MaxDepth >= 0 && depth == rw.MaxDepth
	overMaxDepth := rw.MaxDepth >= 0 && depth > rw.MaxDepth

	// Shortcut when we are over max depth. In practice, this
	// only applies when calling refs with --maxDepth=0, as root's
	// children are already over max depth. Otherwise nothing should
	// hit this.
	if overMaxDepth {
		return false, false
	}

	// We can shortcut right away if we don't need unique output:
	//   - we keep traversing when not atMaxDepth
	//   - always print
	if !rw.Unique {
		return !atMaxDepth, true
	}

	// Unique == true from this point.
	// Thus, we keep track of seen Cids, and their depth.
	if rw.seen == nil {
		rw.seen = make(map[string]int)
	}
	key := string(c.Bytes())
	oldDepth, ok := rw.seen[key]

	// Unique == true && depth < MaxDepth (or unlimited) from this point

	// Branch pruning cases:
	// - We saw the Cid before and either:
	//   - Depth is unlimited (MaxDepth = -1)
	//   - We saw it higher (smaller depth) in the DAG (means we must have
	//     explored deep enough before)
	// Because we saw the CID, we don't print it again.
	if ok && (rw.MaxDepth < 0 || oldDepth <= depth) {
		return false, false
	}

	// Final case, we must keep exploring the DAG from this CID
	// (unless we hit the depth limit).
	// We note down its depth because it was either not seen
	// or is lower than last time.
	// We print if it was not seen.
	rw.seen[key] = depth
	return !atMaxDepth, !ok
}

// Write one edge
func (rw *RefWriter) WriteEdge(from, to cid.Cid, linkname string, enc cidenc.Encoder) error {
	if rw.Ctx != nil {
		select {
		case <-rw.Ctx.Done(): // just in case.
			return rw.Ctx.Err()
		default:
		}
	}

	var s string
	switch {
	case rw.PrintFmt != "":
		s = rw.PrintFmt
		s = strings.Replace(s, "<src>", enc.Encode(from), -1)
		s = strings.Replace(s, "<dst>", enc.Encode(to), -1)
		s = strings.Replace(s, "<linkname>", linkname, -1)
	default:
		s += enc.Encode(to)
	}

	return rw.res.Emit(&RefWrapper{Ref: s})
}
