package commands

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/fileshelpers"

	"github.com/dustin/go-humanize"
	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	dag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-mfs"
	iface "github.com/ipfs/interface-go-ipfs-core"
	mh "github.com/multiformats/go-multihash"
)

// FilesCmd is the 'ipfs files' command
var FilesCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with unixfs files.",
		ShortDescription: `
Files is an API for manipulating IPFS objects as if they were a unix
filesystem.

NOTE:
Most of the subcommands of 'ipfs files' accept the '--flush' flag. It defaults
to true. Use caution when setting this flag to false. It will improve
performance for large numbers of file operations, but it does so at the cost
of consistency guarantees. If the daemon is unexpectedly killed before running
'ipfs files flush' on the files in question, then data may be lost. This also
applies to running 'ipfs repo gc' concurrently with '--flush=false'
operations.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption(filesFlushOptionName, "f", "Flush target and ancestors after write.").WithDefault(true),
	},
	Subcommands: map[string]*cmds.Command{
		"read":  filesReadCmd,
		"write": filesWriteCmd,
		"mv":    filesMvCmd,
		"cp":    filesCpCmd,
		"ls":    filesLsCmd,
		"mkdir": filesMkdirCmd,
		"stat":  filesStatCmd,
		"rm":    filesRmCmd,
		"flush": filesFlushCmd,
		"chcid": filesChcidCmd,
	},
}

const (
	filesCidVersionOptionName = "cid-version"
	filesHashOptionName       = "hash"
)

var cidVersionOption = cmds.IntOption(filesCidVersionOptionName, "cid-ver", "Cid version to use. (experimental)")
var hashOption = cmds.StringOption(filesHashOptionName, "Hash function to use. Will set Cid version to 1 if used. (experimental)")

var errFormat = errors.New("format was set by multiple options. Only one format option is allowed")

const (
	defaultStatFormat = `<hash>
Size: <size>
CumulativeSize: <cumulsize>
ChildBlocks: <childs>
Type: <type>`
	filesFormatOptionName    = "format"
	filesSizeOptionName      = "size"
	filesWithLocalOptionName = "with-local"
)

var filesStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Display file status.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "Path to node to stat."),
	},
	Options: []cmds.Option{
		cmds.StringOption(filesFormatOptionName, "Print statistics in given format. Allowed tokens: "+
			"<hash> <size> <cumulsize> <type> <childs>. Conflicts with other format options.").WithDefault(defaultStatFormat),
		cmds.BoolOption(filesHashOptionName, "Print only hash. Implies '--format=<hash>'. Conflicts with other format options."),
		cmds.BoolOption(filesSizeOptionName, "Print only size. Implies '--format=<cumulsize>'. Conflicts with other format options."),
		cmds.BoolOption(filesWithLocalOptionName, "Compute the amount of the dag that is local, and if possible the total size"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {

		_, err := statGetFormatOptions(req)
		if err != nil {
			return cmds.Errorf(cmds.ErrClient, err.Error())
		}

		node, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		withLocal, _ := req.Options[filesWithLocalOptionName].(bool)

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		o, err := fileshelpers.Stat(
			req.Context,
			node.FilesRoot,
			api,
			node.Blockstore,
			node.DAG,
			req.Arguments[0],
			&iface.FilesStatOptions{
				WithLocality: withLocal,
				CidEncoder:   enc,
			},
		)

		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, o)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *iface.FileInfo) error {
			s, _ := statGetFormatOptions(req)
			s = strings.Replace(s, "<hash>", out.Hash, -1)
			s = strings.Replace(s, "<size>", fmt.Sprintf("%d", out.Size), -1)
			s = strings.Replace(s, "<cumulsize>", fmt.Sprintf("%d", out.CumulativeSize), -1)
			s = strings.Replace(s, "<childs>", fmt.Sprintf("%d", out.Blocks), -1)
			s = strings.Replace(s, "<type>", out.Type, -1)

			fmt.Fprintln(w, s)

			if out.WithLocality {
				fmt.Fprintf(w, "Local: %s of %s (%.2f%%)\n",
					humanize.Bytes(out.SizeLocal),
					humanize.Bytes(out.CumulativeSize),
					100.0*float64(out.SizeLocal)/float64(out.CumulativeSize),
				)
			}

			return nil
		}),
	},
	Type: iface.FileInfo{},
}

func moreThanOne(a, b, c bool) bool {
	return a && b || b && c || a && c
}

func statGetFormatOptions(req *cmds.Request) (string, error) {

	hash, _ := req.Options[filesHashOptionName].(bool)
	size, _ := req.Options[filesSizeOptionName].(bool)
	format, _ := req.Options[filesFormatOptionName].(string)

	if moreThanOne(hash, size, format != defaultStatFormat) {
		return "", errFormat
	}

	if hash {
		return "<hash>", nil
	} else if size {
		return "<cumulsize>", nil
	} else {
		return format, nil
	}
}

var filesCpCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Copy files into mfs.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("source", true, false, "Source object to copy."),
		cmds.StringArg("dest", true, false, "Destination to copy object to."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		flush, _ := req.Options[filesFlushOptionName].(bool)

		return fileshelpers.Copy(req.Context, nd.FilesRoot, api, req.Arguments[0], req.Arguments[1], &iface.FilesCopyOptions{Flush: flush})
	},
}

type filesLsOutput struct {
	Entries []mfs.NodeListing
}

const (
	longOptionName     = "long"
	dontSortOptionName = "U"
)

var filesLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List directories in the local mutable namespace.",
		ShortDescription: `
List directories in the local mutable namespace.

Examples:

    $ ipfs files ls /welcome/docs/
    about
    contact
    help
    quick-start
    readme
    security-notes

    $ ipfs files ls /myfiles/a/b/c/d
    foo
    bar
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", false, false, "Path to show listing for. Defaults to '/'."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(longOptionName, "l", "Use long listing format."),
		cmds.BoolOption(dontSortOptionName, "Do not sort; list entries in directory order."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		var arg string

		if len(req.Arguments) == 0 {
			arg = "/"
		} else {
			arg = req.Arguments[0]
		}

		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		long, _ := req.Options[longOptionName].(bool)

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		listing, err := fileshelpers.List(req.Context, nd.FilesRoot, arg, &iface.FilesListOptions{
			Long:       long,
			CidEncoder: enc,
		})

		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &filesLsOutput{listing})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *filesLsOutput) error {
			noSort, _ := req.Options[dontSortOptionName].(bool)
			if !noSort {
				sort.Slice(out.Entries, func(i, j int) bool {
					return strings.Compare(out.Entries[i].Name, out.Entries[j].Name) < 0
				})
			}

			long, _ := req.Options[longOptionName].(bool)
			for _, o := range out.Entries {
				if long {
					if o.Type == int(mfs.TDir) {
						o.Name += "/"
					}
					fmt.Fprintf(w, "%s\t%s\t%d\n", o.Name, o.Hash, o.Size)
				} else {
					fmt.Fprintf(w, "%s\n", o.Name)
				}
			}

			return nil
		}),
	},
	Type: filesLsOutput{},
}

const (
	filesOffsetOptionName = "offset"
	filesCountOptionName  = "count"
)

var filesReadCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Read a file in a given mfs.",
		ShortDescription: `
Read a specified number of bytes from a file at a given offset. By default,
will read the entire file similar to unix cat.

Examples:

    $ ipfs files read /test/hello
    hello
        `,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "Path to file to be read."),
	},
	Options: []cmds.Option{
		cmds.Int64Option(filesOffsetOptionName, "o", "Byte offset to begin reading from."),
		cmds.Int64Option(filesCountOptionName, "n", "Maximum number of bytes to read (0 reads everything)."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		offset, _ := req.Options[offsetOptionName].(int64)
		count, _ := req.Options[filesCountOptionName].(int64)

		rc, err := fileshelpers.Read(req.Context, nd.FilesRoot, req.Arguments[0], &iface.FilesReadOptions{
			Offset: offset,
			Count:  count,
		})
		if err != nil {
			return err
		}
		defer rc.Close()

		return res.Emit(rc)
	},
}

var filesMvCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Move files.",
		ShortDescription: `
Move files around. Just like traditional unix mv.

Example:

    $ ipfs files mv /myfs/a/b/c /myfs/foo/newc

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("source", true, false, "Source file to move."),
		cmds.StringArg("dest", true, false, "Destination path for file to be moved to."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		flush, _ := req.Options[filesFlushOptionName].(bool)

		return fileshelpers.Move(req.Context, nd.FilesRoot, req.Arguments[0], req.Arguments[1], &iface.FilesMoveOptions{Flush: flush})
	},
}

const (
	filesCreateOptionName    = "create"
	filesParentsOptionName   = "parents"
	filesTruncateOptionName  = "truncate"
	filesRawLeavesOptionName = "raw-leaves"
	filesFlushOptionName     = "flush"
)

var filesWriteCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Write to a mutable file in a given filesystem.",
		ShortDescription: `
Write data to a file in a given filesystem. This command allows you to specify
a beginning offset to write to. The entire length of the input will be
written.

If the '--create' option is specified, the file will be created if it does not
exist. Nonexistant intermediate directories will not be created.

Newly created files will have the same CID version and hash function of the
parent directory unless the --cid-version and --hash options are used.

Newly created leaves will be in the legacy format (Protobuf) if the
CID version is 0, or raw is the CID version is non-zero.  Use of the
--raw-leaves option will override this behavior.

If the '--flush' option is set to false, changes will not be propogated to the
merkledag root. This can make operations much faster when doing a large number
of writes to a deeper directory structure.

EXAMPLE:

    echo "hello world" | ipfs files write --create /myfs/a/b/file
    echo "hello world" | ipfs files write --truncate /myfs/a/b/file

WARNING:

Usage of the '--flush=false' option does not guarantee data durability until
the tree has been flushed. This can be accomplished by running 'ipfs files
stat' on the file or any of its ancestors.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "Path to write to."),
		cmds.FileArg("data", true, false, "Data to write.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.Int64Option(filesOffsetOptionName, "o", "Byte offset to begin writing at."),
		cmds.BoolOption(filesCreateOptionName, "e", "Create the file if it does not exist."),
		cmds.BoolOption(filesParentsOptionName, "p", "Make parent directories as needed."),
		cmds.BoolOption(filesTruncateOptionName, "t", "Truncate the file to size zero before writing."),
		cmds.Int64Option(filesCountOptionName, "n", "Maximum number of bytes to read."),
		cmds.BoolOption(filesRawLeavesOptionName, "Use raw blocks for newly created leaf nodes. (experimental)"),
		cidVersionOption,
		hashOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) (retErr error) {
		create, _ := req.Options[filesCreateOptionName].(bool)
		mkParents, _ := req.Options[filesParentsOptionName].(bool)
		trunc, _ := req.Options[filesTruncateOptionName].(bool)
		flush, _ := req.Options[filesFlushOptionName].(bool)
		rawLeaves, rawLeavesDef := req.Options[filesRawLeavesOptionName].(bool)

		prefix, err := getPrefix(req)
		if err != nil {
			return err
		}

		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		var r io.Reader
		r, err = cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}

		return fileshelpers.Write(req.Context, nd.FilesRoot, req.Arguments[0], r, &iface.FilesWriteOptions{
			Create:            create,
			MakeParents:       mkParents,
			Truncate:          trunc,
			Flush:             flush,
			RawLeaves:         rawLeaves,
			RawLeavesOverride: rawLeavesDef,
			CidBuilder:        prefix,
		})
	},
}

var filesMkdirCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Make directories.",
		ShortDescription: `
Create the directory if it does not already exist.

The directory will have the same CID version and hash function of the
parent directory unless the --cid-version and --hash options are used.

NOTE: All paths must be absolute.

Examples:

    $ ipfs files mkdir /test/newdir
    $ ipfs files mkdir -p /test/does/not/exist/yet
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, "Path to dir to make."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(filesParentsOptionName, "p", "No error if existing, make parent directories as needed."),
		cidVersionOption,
		hashOption,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		dashp, _ := req.Options[filesParentsOptionName].(bool)
		flush, _ := req.Options[filesFlushOptionName].(bool)

		prefix, err := getPrefix(req)
		if err != nil {
			return err
		}

		return fileshelpers.Mkdir(req.Context, n.FilesRoot, req.Arguments[0], &iface.FilesMkdirOptions{
			MakeParents: dashp,
			Flush:       flush,
			CidBuilder:  prefix,
		})
	},
}

type flushRes struct {
	Cid string
}

var filesFlushCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Flush a given path's data to disk.",
		ShortDescription: `
Flush a given path to disk. This is only useful when other commands
are run with the '--flush=false'.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", false, false, "Path to flush. Default: '/'."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		path := "/"
		if len(req.Arguments) > 0 {
			path = req.Arguments[0]
		}

		n, err := mfs.FlushPath(req.Context, nd.FilesRoot, path)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &flushRes{enc.Encode(n.Cid())})
	},
	Type: flushRes{},
}

var filesChcidCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Change the cid version or hash function of the root node of a given path.",
		ShortDescription: `
Change the cid version or hash function of the root node of a given path.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", false, false, "Path to change. Default: '/'."),
	},
	Options: []cmds.Option{
		cidVersionOption,
		hashOption,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		path := "/"
		if len(req.Arguments) > 0 {
			path = req.Arguments[0]
		}

		flush, _ := req.Options[filesFlushOptionName].(bool)
		prefix, err := getPrefix(req)
		if err != nil {
			return err
		}

		return fileshelpers.ChangeCid(req.Context, nd.FilesRoot, path, &iface.FilesChangeCidOptions{
			CidBuilder: prefix,
			Flush:      flush,
		})
	},
}

var filesRmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove a file.",
		ShortDescription: `
Remove files or directories.

    $ ipfs files rm /foo
    $ ipfs files ls /bar
    cat
    dog
    fish
    $ ipfs files rm -r /bar
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, true, "File to remove."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(recursiveOptionName, "r", "Recursively remove directories."),
		cmds.BoolOption(forceOptionName, "Forcibly remove target at path; implies -r for directories"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		force, _ := req.Options[forceOptionName].(bool)
		recursive, _ := req.Options[recursiveOptionName].(bool)

		return fileshelpers.Remove(req.Context, nd.FilesRoot, req.Arguments[0], &iface.FilesRemoveOptions{
			Force:     force,
			Recursive: recursive,
		})
	},
}

func getPrefix(req *cmds.Request) (cid.Builder, error) {
	cidVer, cidVerSet := req.Options[filesCidVersionOptionName].(int)
	hashFunStr, hashFunSet := req.Options[filesHashOptionName].(string)

	if !cidVerSet && !hashFunSet {
		return nil, nil
	}

	if hashFunSet && cidVer == 0 {
		cidVer = 1
	}

	prefix, err := dag.PrefixForCidVersion(cidVer)
	if err != nil {
		return nil, err
	}

	if hashFunSet {
		hashFunCode, ok := mh.Names[strings.ToLower(hashFunStr)]
		if !ok {
			return nil, fmt.Errorf("unrecognized hash function: %s", strings.ToLower(hashFunStr))
		}
		prefix.MhType = hashFunCode
		prefix.MhLength = -1
	}

	return &prefix, nil
}
