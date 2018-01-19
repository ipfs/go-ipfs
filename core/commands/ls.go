package commands

import (
	"bytes"
	"fmt"
	"io"
	"text/tabwriter"

	blockservice "github.com/ipfs/go-ipfs/blockservice"
	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	unixfs "github.com/ipfs/go-ipfs/unixfs"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	unixfspb "github.com/ipfs/go-ipfs/unixfs/pb"

	node "gx/ipfs/QmNwUEK7QbwSqyKBu3mMtToo8SUc6wQJ7gdZq4gGGJqfnf/go-ipld-format"
	"gx/ipfs/QmQp2a2Hhb7F6eK2A5hN8f9aJy4mtkEikL9Zj4cgB7d1dD/go-ipfs-cmdkit"
)

type LsLink struct {
	Name, Hash string
	Size       uint64
	Type       unixfspb.Data_DataType
}

type LsObject struct {
	Hash  string
	Links []LsLink
}

type LsOutput struct {
	Objects []LsObject
}

var LsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List directory contents for Unix filesystem objects.",
		ShortDescription: `
Displays the contents of an IPFS or IPNS object(s) at the given path, with
the following format:

  <link base58 hash> <link size in bytes> <link name>

The JSON output contains type information.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to list links from.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("headers", "v", "Print table headers (Hash, Size, Name)."),
		cmdkit.BoolOption("resolve-type", "Resolve linked objects to find out their types.").WithDefault(true),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		// get options early -> exit early in case of error
		if _, _, err := req.Option("headers").Bool(); err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		resolve, _, err := req.Option("resolve-type").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		dserv := nd.DAG
		if !resolve {
			offlineexch := offline.Exchange(nd.Blockstore)
			bserv := blockservice.New(nd.Blockstore, offlineexch)
			dserv = merkledag.NewDAGService(bserv, nd.Repo.Datastore())
		}

		paths := req.Arguments()

		var dagnodes []node.Node
		for _, fpath := range paths {
			p, err := path.ParsePath(fpath)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			r := &path.Resolver{
				DAG:         nd.DAG,
				ResolveOnce: uio.ResolveUnixfsOnce,
			}

			dagnode, err := core.Resolve(req.Context(), nd.Namesys, r, p)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
			dagnodes = append(dagnodes, dagnode)
		}

		output := make([]LsObject, len(req.Arguments()))

		for i, dagnode := range dagnodes {
			dir, err := uio.NewDirectoryFromNode(nd.DAG, dagnode)
			if err != nil && err != uio.ErrNotADir {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			var links []*node.Link
			if dir == nil {
				links = dagnode.Links()
			} else {
				links, err = dir.Links(req.Context())
				if err != nil {
					res.SetError(err, cmdkit.ErrNormal)
					return
				}
			}

			output[i] = LsObject{
				Hash:  paths[i],
				Links: make([]LsLink, len(links)),
			}

			for j, link := range links {
				t := unixfspb.Data_DataType(-1)

				linkNode, err := link.GetNode(req.Context(), dserv)
				if err == merkledag.ErrNotFound && !resolve {
					// not an error
					linkNode = nil
				} else if err != nil {
					res.SetError(err, cmdkit.ErrNormal)
					return
				}

				if pn, ok := linkNode.(*merkledag.ProtoNode); ok {
					d, err := unixfs.FromBytes(pn.Data())
					if err != nil {
						res.SetError(err, cmdkit.ErrNormal)
						return
					}

					t = d.GetType()
				}
				output[i].Links[j] = LsLink{
					Name: link.Name,
					Hash: link.Cid.String(),
					Size: link.Size,
					Type: t,
				}
			}
		}

		res.SetOutput(&LsOutput{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {

			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			headers, _, _ := res.Request().Option("headers").Bool()
			output, ok := v.(*LsOutput)
			if !ok {
				return nil, e.TypeErr(output, v)
			}

			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)
			for _, object := range output.Objects {
				if len(output.Objects) > 1 {
					fmt.Fprintf(w, "%s:\n", object.Hash)
				}
				if headers {
					fmt.Fprintln(w, "Hash\tSize\tName")
				}
				for _, link := range object.Links {
					if link.Type == unixfspb.Data_Directory {
						link.Name += "/"
					}
					fmt.Fprintf(w, "%s\t%v\t%s\n", link.Hash, link.Size, link.Name)
				}
				if len(output.Objects) > 1 {
					fmt.Fprintln(w)
				}
			}
			w.Flush()

			return buf, nil
		},
	},
	Type: LsOutput{},
}
