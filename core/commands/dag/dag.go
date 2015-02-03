package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	ccutil "github.com/jbenet/go-ipfs/core/commands/util"
	path "github.com/jbenet/go-ipfs/path"
	dag "github.com/jbenet/go-ipfs/struct/merkledag"
)

var log = eventlog.Logger("core/cmds/object")

// ErrObjectTooLarge is returned when too much data was read from stdin. current limit 512k
var ErrObjectTooLarge = errors.New("input object was too large. limit is 512kbytes")

const inputLimit = 512 * 1024

type Node struct {
	Links []ccutil.Link
	Data  []byte
}

var DagCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "manipulate ipfs merkledag",
		ShortDescription: `
'ipfs dag' is a plumbing command used to manipulate DAG objects
directly.`,
		Synopsis: `
ipfs dag get <key>             - Get the DAG node named by <key>
ipfs dag put <data> <encoding> - Stores input, outputs its key
ipfs dag data <key>            - Outputs raw bytes in an object
ipfs dag links <key>           - Outputs links pointed to by object
ipfs dag stat <key>            - Outputs statistics of object
`,
	},

	Subcommands: map[string]*cmds.Command{
		"data":  dagDataCmd,
		"links": dagLinksCmd,
		"get":   dagGetCmd,
		"put":   dagPutCmd,
		"stat":  dagStatCmd,
	},
}

var dagDataCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Outputs the raw bytes in an IPFS node",
		ShortDescription: `
ipfs data is a plumbing command for retreiving the raw bytes stored in
a DAG node. It outputs to stdout, and <key> is a base58 encoded
multihash.
`,
		LongDescription: `
ipfs data is a plumbing command for retreiving the raw bytes stored in
a DAG node. It outputs to stdout, and <key> is a base58 encoded
multihash.

Note that the "--encoding" option does not affect the output, since the
output is the raw data of the object.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve (base58-encoded multihash)").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fpath := path.Path(req.Arguments()[0])
		output, err := objectData(n, fpath)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(output)
	},
}

var dagLinksCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Outputs links pointed to by given object",
		ShortDescription: `
'ipfs dag links' is a plumbing command for retreiving the links from
a DAG node. It outputs to stdout, and <key> is a base58 encoded
multihash.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve (base58-encoded multihash)").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fpath := path.Path(req.Arguments()[0])
		output, err := objectLinks(n, fpath)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(output)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			object := res.Output().(*ccutil.Object)
			marshalled := ccutil.MarshalLinks(object.Links)
			return strings.NewReader(marshalled), nil
		},
	},
	Type: ccutil.Object{},
}

var dagGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get and serialize the DAG node named by <key>",
		ShortDescription: `
'ipfs dag get' is a plumbing command for retreiving DAG nodes.
It serializes the DAG node to the format specified by the "--encoding"
flag. It outputs to stdout, and <key> is a base58 encoded multihash.
`,
		LongDescription: `
'ipfs dag get' is a plumbing command for retreiving DAG nodes.
It serializes the DAG node to the format specified by the "--encoding"
flag. It outputs to stdout, and <key> is a base58 encoded multihash.

This command outputs data in the following encodings:
  * "protobuf"
  * "json"
  * "xml"
(Specified by the "--encoding" or "-enc" flag)`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve (base58-encoded multihash)").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fpath := path.Path(req.Arguments()[0])

		object, err := objectGet(n, fpath)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		node := &Node{
			Links: make([]ccutil.Link, len(object.Links)),
			Data:  object.Data,
		}

		for i, link := range object.Links {
			node.Links[i] = ccutil.Link{
				Hash: link.Hash.B58String(),
				Name: link.Name,
				Size: link.Size,
			}
		}

		res.SetOutput(node)
	},
	Type: Node{},
	Marshalers: cmds.MarshalerMap{
		cmds.EncodingType("protobuf"): func(res cmds.Response) (io.Reader, error) {
			node := res.Output().(*Node)
			object, err := deserializeNode(node)
			if err != nil {
				return nil, err
			}

			marshaled, err := object.Marshal()
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(marshaled), nil
		},
	},
}

var dagStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get stats for the DAG node named by <key>",
		ShortDescription: `
'ipfs dag stat' is a plumbing command to print DAG node statistics.
<key> is a base58 encoded multihash. It outputs to stdout:

	NumLinks        int number of links in link table
	BlockSize       int size of the raw, encoded data
	LinksSize       int size of the links segment
	DataSize        int size of the data segment
	CumulativeSize  int cumulative size of object and its references
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve (base58-encoded multihash)").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fpath := path.Path(req.Arguments()[0])

		object, err := objectGet(n, fpath)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		ns, err := object.Stat()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(ns)
	},
	Type: dag.NodeStat{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			ns := res.Output().(*dag.NodeStat)

			var buf bytes.Buffer
			w := func(s string, n int) {
				buf.Write([]byte(fmt.Sprintf("%s: %d\n", s, n)))
			}
			w("NumLinks", ns.NumLinks)
			w("BlockSize", ns.BlockSize)
			w("LinksSize", ns.LinksSize)
			w("DataSize", ns.DataSize)
			w("CumulativeSize", ns.CumulativeSize)

			return &buf, nil
		},
	},
}

var dagPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Stores input as a DAG object, outputs its key",
		ShortDescription: `
'ipfs dag put' is a plumbing command for storing DAG nodes.
It reads from stdin, and the output is a base58 encoded multihash.
`,
		LongDescription: `
'ipfs dag put' is a plumbing command for storing DAG nodes.
It reads from stdin, and the output is a base58 encoded multihash.

Data should be in the format specified by <encoding>.
<encoding> may be one of the following:
	* "protobuf"
	* "json"
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, false, "Data to be stored as a DAG object"),
		cmds.StringArg("encoding", true, false, "Encoding type of <data>, either \"protobuf\" or \"json\""),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		input, err := req.Files().NextFile()
		if err != nil && err != io.EOF {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		encoding := req.Arguments()[0]

		output, err := objectPut(n, input, encoding)
		if err != nil {
			errType := cmds.ErrNormal
			if err == ErrUnknownObjectEnc {
				errType = cmds.ErrClient
			}
			res.SetError(err, errType)
			return
		}

		res.SetOutput(output)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			object := res.Output().(*ccutil.Object)
			return strings.NewReader("added " + object.Hash), nil
		},
	},
	Type: ccutil.Object{},
}

// objectData takes a key string and writes out the raw bytes of that node (if there is one)
func objectData(n *core.IpfsNode, fpath path.Path) (io.Reader, error) {
	dagnode, err := n.Resolver.ResolvePath(fpath)
	if err != nil {
		return nil, err
	}

	log.Debugf("objectData: found dagnode %s (# of bytes: %d - # links: %d)", fpath, len(dagnode.Data), len(dagnode.Links))

	return bytes.NewReader(dagnode.Data), nil
}

// objectLinks takes a key string and lists the links it points to
func objectLinks(n *core.IpfsNode, fpath path.Path) (*ccutil.Object, error) {
	dagnode, err := n.Resolver.ResolvePath(fpath)
	if err != nil {
		return nil, err
	}

	log.Debugf("objectLinks: found dagnode %s (# of bytes: %d - # links: %d)", fpath, len(dagnode.Data), len(dagnode.Links))

	return ccutil.NodeOutput(dagnode)
}

// objectGet takes a key string from args and a format option and serializes the dagnode to that format
func objectGet(n *core.IpfsNode, fpath path.Path) (*dag.Node, error) {
	dagnode, err := n.Resolver.ResolvePath(fpath)
	if err != nil {
		return nil, err
	}

	log.Debugf("objectGet: found dagnode %s (# of bytes: %d - # links: %d)", fpath, len(dagnode.Data), len(dagnode.Links))

	return dagnode, nil
}

// objectPut takes a format option, serializes bytes from stdin and updates the dag with that data
func objectPut(n *core.IpfsNode, input io.Reader, encoding string) (*ccutil.Object, error) {
	var (
		dagnode *dag.Node
		data    []byte
		err     error
	)

	data, err = ioutil.ReadAll(io.LimitReader(input, inputLimit+10))
	if err != nil {
		return nil, err
	}

	if len(data) >= inputLimit {
		return nil, ErrObjectTooLarge
	}

	switch getObjectEnc(encoding) {
	case objectEncodingJSON:
		node := new(Node)
		err = json.Unmarshal(data, node)
		if err != nil {
			return nil, err
		}

		dagnode, err = deserializeNode(node)
		if err != nil {
			return nil, err
		}

	case objectEncodingProtobuf:
		dagnode, err = dag.Decoded(data)

	default:
		return nil, ErrUnknownObjectEnc
	}

	if err != nil {
		return nil, err
	}

	if err := ccutil.AddNode(n, dagnode); err != nil {
		return nil, err
	}

	return ccutil.NodeOutput(dagnode)
}

// ErrUnknownObjectEnc is returned if a invalid encoding is supplied
var ErrUnknownObjectEnc = errors.New("unknown object encoding")

type objectEncoding string

const (
	objectEncodingJSON     objectEncoding = "json"
	objectEncodingProtobuf                = "protobuf"
)

func getObjectEnc(o interface{}) objectEncoding {
	v, ok := o.(string)
	if !ok {
		// chosen as default because it's human readable
		log.Warning("option is not a string - falling back to json")
		return objectEncodingJSON
	}

	return objectEncoding(v)
}

// converts the Node object into a real dag.Node
func deserializeNode(node *Node) (*dag.Node, error) {
	dagnode := new(dag.Node)
	dagnode.Data = node.Data
	dagnode.Links = make([]*dag.Link, len(node.Links))
	for i, link := range node.Links {
		hash, err := mh.FromB58String(link.Hash)
		if err != nil {
			return nil, err
		}
		dagnode.Links[i] = &dag.Link{
			Name: link.Name,
			Size: link.Size,
			Hash: hash,
		}
	}

	return dagnode, nil
}
