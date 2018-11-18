package commands

import (
	"bytes"
	"fmt"
	"io"
	"text/tabwriter"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	iface "github.com/ipfs/go-ipfs/core/coreapi/interface"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	unixfs "gx/ipfs/QmUnHNqhSB1JgzVCxL1Kz3yb4bdyB4q1Z9AD5AUBVmt3fZ/go-unixfs"
	uio "gx/ipfs/QmUnHNqhSB1JgzVCxL1Kz3yb4bdyB4q1Z9AD5AUBVmt3fZ/go-unixfs/io"
	unixfspb "gx/ipfs/QmUnHNqhSB1JgzVCxL1Kz3yb4bdyB4q1Z9AD5AUBVmt3fZ/go-unixfs/pb"
	blockservice "gx/ipfs/QmVDTbzzTwnuBwNbJdhW3u7LoBQp46bezm9yp4z1RoEepM/go-blockservice"
	offline "gx/ipfs/QmYZwey1thDTynSrvd6qQkX24UpTka6TFhQ2v569UpoqxD/go-ipfs-exchange-offline"
	merkledag "gx/ipfs/QmcGt25mrjuB2kKW2zhPbXVZNHc4yoTDQ65NA8m6auP2f1/go-merkledag"
	ipld "gx/ipfs/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

type LsLink struct {
	Name, Hash string
	Size       uint64
	Type       unixfspb.Data_DataType
}

type LsObject struct {
	Hash  string
	Size  uint64
	Type  string
	Links []LsLink
}

type LsOutput struct {
	Objects []LsObject
}

const (
	lsHeadersOptionNameTime = "headers"
	lsResolveTypeOptionName = "resolve-type"
)

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
		cmdkit.BoolOption(lsHeadersOptionNameTime, "v", "Print table headers (Hash, Size, Name)."),
		cmdkit.BoolOption(lsResolveTypeOptionName, "Resolve linked objects to find out their types.").WithDefault(true),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		// get options early -> exit early in case of error
		if _, _, err := req.Option(lsHeadersOptionNameTime).Bool(); err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		resolve, _, err := req.Option(lsResolveTypeOptionName).Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		dserv := nd.DAG
		if !resolve {
			offlineexch := offline.Exchange(nd.Blockstore)
			bserv := blockservice.New(nd.Blockstore, offlineexch)
			dserv = merkledag.NewDAGService(bserv)
		}

		paths := req.Arguments()

		var dagnodes []ipld.Node
		for _, fpath := range paths {
			p, err := iface.ParsePath(fpath)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			dagnode, err := api.ResolveNode(req.Context(), p)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
			dagnodes = append(dagnodes, dagnode)
		}

		output := make([]LsObject, len(req.Arguments()))
		ng := merkledag.NewSession(req.Context(), nd.DAG)
		ro := merkledag.NewReadOnlyDagService(ng)

		for i, dagnode := range dagnodes {
			dagp, ok := dagnode.(*merkledag.ProtoNode)
			if !ok {
				res.SetError(merkledag.ErrNotProtobuf, cmdkit.ErrNormal)
				return
			}

			fsn, err := unixfs.FSNodeFromBytes(dagp.Data())
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			var links []*ipld.Link

			switch fsn.Type() {
			case unixfs.TSymlink:
				res.SetError(fmt.Errorf("cannot list symlinks yet"), cmdkit.ErrNormal)
				return
			case unixfs.THAMTShard:
				res.SetError(fmt.Errorf("cannot list large directories yet"), cmdkit.ErrNormal)
				return
			case unixfs.TFile:
				links = dagnode.Links()
				output[i] = LsObject{
					Hash:  paths[i],
					Size:  fsn.FileSize(),
					Links: make([]LsLink, len(links)),
				}
			case unixfs.TDirectory:
				dir, err := uio.NewDirectoryFromNode(ro, dagnode)
				if err != nil {
					res.SetError(fmt.Errorf("the data in %s (at %q) is not a UnixFS directory: %s", dagnode.Cid(), paths[i], err), cmdkit.ErrNormal)
					return
				}
				links, err = dir.Links(req.Context())
				if err != nil {
					res.SetError(err, cmdkit.ErrNormal)
					return
				}

				output[i] = LsObject{
					Hash:  paths[i],
					Type:  fsn.Type().String(),
					Links: make([]LsLink, len(links)),
				}
			default:
				res.SetError(fmt.Errorf("unrecognized type: %s", fsn.Type()), cmdkit.ErrImplementation)
				return
			}

			for j, link := range links {
				t := unixfspb.Data_DataType(-1)
				size := uint64(0)
				switch link.Cid.Type() {
				case cid.Raw:
					// No need to check with raw leaves
					t = unixfs.TFile
					size = link.Size
				case cid.DagProtobuf:
					linkNode, err := link.GetNode(req.Context(), dserv)
					if err == ipld.ErrNotFound && !resolve {
						// not an error
						linkNode = nil
					} else if err != nil {
						res.SetError(err, cmdkit.ErrNormal)
						return
					}

					if pn, ok := linkNode.(*merkledag.ProtoNode); ok {
						fsn, err := unixfs.FSNodeFromBytes(pn.Data())
						t = fsn.Type()
						if err != nil {
							res.SetError(err, cmdkit.ErrNormal)
							return
						}
						if t == unixfs.TFile {
							size = fsn.FileSize()
						} else {
							size = link.Size
						}
					}
				}
				output[i].Links[j] = LsLink{
					Name: link.Name,
					Hash: link.Cid.String(),
					Size: size,
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

			headers, _, _ := res.Request().Option(lsHeadersOptionNameTime).Bool()
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

				// file object
				if object.Hash != "" && object.Links == nil {
					fmt.Fprintf(w, "%s\t%v\t\n", object.Hash, object.Size)
				}

				for _, link := range object.Links {
					if link.Type == unixfs.TDirectory {
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
