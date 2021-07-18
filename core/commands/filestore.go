package commands

import (
	"fmt"
	"io"
	"os"

	filestore "github.com/ipfs/go-filestore"
	core "github.com/ipfs/go-ipfs/core"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmds"
)

var FileStoreCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with filestore objects.",
	},
	Subcommands: map[string]*cmds.Command{
		"ls":     lsFileStore,
		"verify": verifyFileStore,
		"dups":   dupsFileStore,
	},
}

const (
	fileOrderOptionName       = "file-order"
	removeBadBlocksOptionName = "remove-bad-blocks"
)

var lsFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects in filestore.",
		LongDescription: `
List objects in the filestore.

If one or more <obj> is specified only list those specific objects,
otherwise list all objects.

The output is:

<hash> <size> <path> <offset>
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("obj", false, true, "Cid of objects to list."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(fileOrderOptionName, "sort the results based on the path of the backing file"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		_, fs, err := getFilestore(env)
		if err != nil {
			return err
		}
		args := req.Arguments
		if len(args) > 0 {
			return listByArgs(res, fs, args, false)
		}

		fileOrder, _ := req.Options[fileOrderOptionName].(bool)
		next, err := filestore.ListAll(fs, fileOrder)
		if err != nil {
			return err
		}

		for {
			r := next()
			if r == nil {
				break
			}
			if err := res.Emit(r); err != nil {
				return err
			}
		}

		return nil
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			enc, err := cmdenv.GetCidEncoder(res.Request())
			if err != nil {
				return err
			}
			return streamResult(func(v interface{}, out io.Writer) nonFatalError {
				r := v.(*filestore.ListRes)
				if r.ErrorMsg != "" {
					return nonFatalError(r.ErrorMsg)
				}
				fmt.Fprintf(out, "%s\n", r.FormatLong(enc.Encode))
				return ""
			})(res, re)
		},
	},
	Type: filestore.ListRes{},
}

var verifyFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Verify objects in filestore.",
		LongDescription: `
Verify objects in the filestore.

If one or more <obj> is specified only verify those specific objects,
otherwise verify all objects.

The output is:

<status> <hash> <size> <path> <offset> <action>

Where <status> is one of:
ok:       the block can be reconstructed
changed:  the contents of the backing file have changed
no-file:  the backing file could not be found
error:    there was some other problem reading the file
missing:  <obj> could not be found in the filestore
ERROR:    internal error, most likely due to a corrupt database

Where <action> is one of:
remove:   link will be removed
nop:      nothing to do with it

For ERROR entries the error will also be printed to stderr.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("obj", false, true, "Cid of objects to verify."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(fileOrderOptionName, "verify the objects based on the order of the backing file"),
		cmds.BoolOption(removeBadBlocksOptionName, "remove bas blocks. WARNING: This may remove pinned data. You should run 'ipfs pin verify' after running this command and correct any issues."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		_, fs, err := getFilestore(env)
		if err != nil {
			return err
		}

		removeBadBlocks, _ := req.Options[removeBadBlocksOptionName].(bool)
		args := req.Arguments
		if len(args) > 0 {
			return listByArgs(res, fs, args, removeBadBlocks)
		}

		fileOrder, _ := req.Options[fileOrderOptionName].(bool)
		next, err := filestore.VerifyAll(fs, fileOrder)
		if err != nil {
			return err
		}

		for {
			r := next()
			if r == nil {
				break
			}

			if removeBadBlocks && (r.Status != filestore.StatusOk) && (r.Status != filestore.StatusOtherError) {
				fs.FileManager().DeleteBlock(r.Key)
			}

			if err := res.Emit(r); err != nil {
				return err
			}
		}

		return nil
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			enc, err := cmdenv.GetCidEncoder(res.Request())
			if err != nil {
				return err
			}

			req := res.Request()
			removeBadBlocks, _ := req.Options[removeBadBlocksOptionName].(bool)
			for {
				v, err := res.Next()
				if err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}

				list, ok := v.(*filestore.ListRes)
				if !ok {
					return e.TypeErr(list, v)
				}

				if list.Status == filestore.StatusOtherError {
					fmt.Fprintf(os.Stderr, "%s\n", list.ErrorMsg)
				}

				action := "nop"
				if removeBadBlocks && (list.Status != filestore.StatusOk) && (list.Status != filestore.StatusOtherError) {
					action = "remove"
				}

				fmt.Fprintf(os.Stdout, "%s %s %s\n", list.Status.Format(), list.FormatLong(enc.Encode), action)
			}
		},
	},
	Type: filestore.ListRes{},
}

var dupsFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List blocks that are both in the filestore and standard block storage.",
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		_, fs, err := getFilestore(env)
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		ch, err := fs.FileManager().AllKeysChan(req.Context)
		if err != nil {
			return err
		}

		for cid := range ch {
			have, err := fs.MainBlockstore().Has(cid)
			if err != nil {
				return res.Emit(&RefWrapper{Err: err.Error()})
			}
			if have {
				if err := res.Emit(&RefWrapper{Ref: enc.Encode(cid)}); err != nil {
					return err
				}
			}
		}

		return nil
	},
	Encoders: refsEncoderMap,
	Type:     RefWrapper{},
}

func getFilestore(env cmds.Environment) (*core.IpfsNode, *filestore.Filestore, error) {
	n, err := cmdenv.GetNode(env)
	if err != nil {
		return nil, nil, err
	}
	fs := n.Filestore
	if fs == nil {
		return n, nil, filestore.ErrFilestoreNotEnabled
	}
	return n, fs, err
}

func listByArgs(res cmds.ResponseEmitter, fs *filestore.Filestore, args []string, removeBadBlocks bool) error {
	for _, arg := range args {
		c, err := cid.Decode(arg)
		if err != nil {
			ret := &filestore.ListRes{
				Status:   filestore.StatusOtherError,
				ErrorMsg: fmt.Sprintf("%s: %v", arg, err),
			}
			if err := res.Emit(ret); err != nil {
				return err
			}
			continue
		}
		r := filestore.Verify(fs, c)

		if removeBadBlocks && (r.Status != filestore.StatusOk) && (r.Status != filestore.StatusOtherError) {
			fs.FileManager().DeleteBlock(r.Key)
		}

		if err := res.Emit(r); err != nil {
			return err
		}
	}

	return nil
}
