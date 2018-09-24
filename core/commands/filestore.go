package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	oldCmds "github.com/ipfs/go-ipfs/commands"
	lgc "github.com/ipfs/go-ipfs/commands/legacy"
	"github.com/ipfs/go-ipfs/core"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	"github.com/ipfs/go-ipfs/filestore"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	cmds "gx/ipfs/QmPXR4tNdLbp8HsZiPMjpsgqphX9Vhw2J6Jh5MKH2ovW3D/go-ipfs-cmds"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
)

var FileStoreCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with filestore objects.",
	},
	Subcommands: map[string]*cmds.Command{
		"ls":     lsFileStore,
		"verify": lgc.NewCommand(verifyFileStore),
		"dups":   lgc.NewCommand(dupsFileStore),
	},
}

var lsFileStore = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List objects in filestore.",
		LongDescription: `
List objects in the filestore.

If one or more <obj> is specified only list those specific objects,
otherwise list all objects.

The output is:

<hash> <size> <path> <offset>
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("obj", false, true, "Cid of objects to list."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("file-order", "sort the results based on the path of the backing file"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		_, fs, err := getFilestore(env)
		if err != nil {
			return err
		}
		args := req.Arguments
		if len(args) > 0 {
			out := perKeyActionToChan(req.Context, args, func(c cid.Cid) *filestore.ListRes {
				return filestore.List(fs, c)
			})

			return res.Emit(out)
		}

		fileOrder, _ := req.Options["file-order"].(bool)
		next, err := filestore.ListAll(fs, fileOrder)
		if err != nil {
			return err
		}

		out := listResToChan(req.Context, next)
		return res.Emit(out)
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			var errors bool
			for {
				v, err := res.Next()
				if err != nil {
					if err == io.EOF {
						break
					}
					return err
				}

				r, ok := v.(*filestore.ListRes)
				if !ok {
					return e.New(e.TypeErr(r, v))
				}

				if r.ErrorMsg != "" {
					errors = true
					fmt.Fprintf(os.Stderr, "%s\n", r.ErrorMsg)
				} else {
					fmt.Fprintf(os.Stdout, "%s\n", r.FormatLong())
				}
			}

			if errors {
				return fmt.Errorf("errors while displaying some entries")
			}

			return nil
		},
	},
	Type: filestore.ListRes{},
}

var verifyFileStore = &oldCmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Verify objects in filestore.",
		LongDescription: `
Verify objects in the filestore.

If one or more <obj> is specified only verify those specific objects,
otherwise verify all objects.

The output is:

<status> <hash> <size> <path> <offset>

Where <status> is one of:
ok:       the block can be reconstructed
changed:  the contents of the backing file have changed
no-file:  the backing file could not be found
error:    there was some other problem reading the file
missing:  <obj> could not be found in the filestore
ERROR:    internal error, most likely due to a corrupt database

For ERROR entries the error will also be printed to stderr.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("obj", false, true, "Cid of objects to verify."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("file-order", "verify the objects based on the order of the backing file"),
	},
	Run: func(req oldCmds.Request, res oldCmds.Response) {
		_, fs, err := getFilestore(req.InvocContext())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		args := req.Arguments()
		if len(args) > 0 {
			out := perKeyActionToChan(req.Context(), args, func(c cid.Cid) *filestore.ListRes {
				return filestore.Verify(fs, c)
			})
			res.SetOutput(out)
		} else {
			fileOrder, _, _ := req.Option("file-order").Bool()
			next, err := filestore.VerifyAll(fs, fileOrder)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
			out := listResToChan(req.Context(), next)
			res.SetOutput(out)
		}
	},
	Marshalers: oldCmds.MarshalerMap{
		oldCmds.Text: func(res oldCmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			r, ok := v.(*filestore.ListRes)
			if !ok {
				return nil, e.TypeErr(r, v)
			}

			if r.Status == filestore.StatusOtherError {
				fmt.Fprintf(res.Stderr(), "%s\n", r.ErrorMsg)
			}
			fmt.Fprintf(res.Stdout(), "%s %s\n", r.Status.Format(), r.FormatLong())
			return nil, nil
		},
	},
	Type: filestore.ListRes{},
}

var dupsFileStore = &oldCmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List blocks that are both in the filestore and standard block storage.",
	},
	Run: func(req oldCmds.Request, res oldCmds.Response) {
		_, fs, err := getFilestore(req.InvocContext())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		ch, err := fs.FileManager().AllKeysChan(req.Context())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		out := make(chan interface{}, 128)
		res.SetOutput((<-chan interface{})(out))

		go func() {
			defer close(out)
			for h := range ch {
				have, err := fs.MainBlockstore().Has(h)
				if err != nil {
					select {
					case out <- &RefWrapper{Err: err.Error()}:
					case <-req.Context().Done():
					}
					return
				}
				if have {
					c := cid.NewCidV1(cid.Raw, h)
					select {
					case out <- &RefWrapper{Ref: c.String()}:
					case <-req.Context().Done():
						return
					}
				}
			}
		}()
	},
	Marshalers: refsMarshallerMap,
	Type:       RefWrapper{},
}

func getFilestore(env interface{}) (*core.IpfsNode, *filestore.Filestore, error) {
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

func listResToChan(ctx context.Context, next func() *filestore.ListRes) <-chan interface{} {
	out := make(chan interface{}, 128)
	go func() {
		defer close(out)
		for {
			r := next()
			if r == nil {
				return
			}
			select {
			case out <- r:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func perKeyActionToChan(ctx context.Context, args []string, action func(cid.Cid) *filestore.ListRes) <-chan interface{} {
	out := make(chan interface{}, 128)
	go func() {
		defer close(out)
		for _, arg := range args {
			c, err := cid.Decode(arg)
			if err != nil {
				select {
				case out <- &filestore.ListRes{
					Status:   filestore.StatusOtherError,
					ErrorMsg: fmt.Sprintf("%s: %v", arg, err),
				}:
				case <-ctx.Done():
				}

				continue
			}
			r := action(c)
			select {
			case out <- r:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
