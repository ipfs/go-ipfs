package dagcmd

import (
	"fmt"
	"io"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"

	cid "github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmds "github.com/ipfs/go-ipfs-cmds"
	ipfspath "github.com/ipfs/go-path"
	//gipfree "github.com/ipld/go-ipld-prime/impl/free"
	//gipselector "github.com/ipld/go-ipld-prime/traversal/selector"
	//gipselectorbuilder "github.com/ipld/go-ipld-prime/traversal/selector/builder"
)

const (
	progressOptionName = "progress"
	silentOptionName   = "silent"
	pinRootsOptionName = "pin-roots"
)

// DagCmd provides a subset of commands for interacting with ipld dag objects
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
		"stat":    DagStatCmd,
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

// RootMeta is the metadata for a root pinning response
type RootMeta struct {
	Cid         cid.Cid
	PinErrorMsg string
}

// DagPutCmd is a command for adding a dag node
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
		cmds.StringOption("format", "f", "Format that the object will be added as.").WithDefault("dag-cbor"),
		cmds.StringOption("input-enc", "Format that the input object will be.").WithDefault("dag-json"),
		cmds.BoolOption("pin", "Pin this object when adding."),
		cmds.StringOption("hash", "Hash function to use").WithDefault(""),
	},
	Run:  dagPut,
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

// DagGetCmd is a command for getting a dag node from IPFS
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
	Run: dagGet,
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
	Run: dagResolve,
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

// DagImportCmd is a command for importing a car to ipfs
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
	Run:  dagImport,
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

// DagExportCmd is a command for exporting an ipfs dag to a car
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
	Run: dagExport,
	PostRun: cmds.PostRunMap{
		cmds.CLI: finishCLIExport,
	},
}

// DagStat is a dag stat command response
type DagStat struct {
	Size      uint64
	NumBlocks int64
}

func (s *DagStat) String() string {
	return fmt.Sprintf("Size: %d, NumBlocks: %d", s.Size, s.NumBlocks)
}

// DagStatCmd is a command for getting size information about an ipfs-stored dag
var DagStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Gets stats for a DAG",
		ShortDescription: `
'ipfs dag stat' fetches a dag and returns various statistics about the DAG.
Statistics include size and number of blocks.

Note: This command skips duplicate blocks in reporting both size and the number of blocks
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "CID of a DAG root to get statistics for").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(progressOptionName, "p", "Return progressive data while reading through the DAG").WithDefault(true),
	},
	Run:  dagStat,
	Type: DagStat{},
	PostRun: cmds.PostRunMap{
		cmds.CLI: finishCLIStat,
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, event *DagStat) error {
			_, err := fmt.Fprintf(
				w,
				"%v\n",
				event,
			)
			return err
		}),
	},
}
