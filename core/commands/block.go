package commands

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/ipfs/go-ipfs/blocks"
	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	util "github.com/ipfs/go-ipfs/blocks/blockstore/util"
	cmds "github.com/ipfs/go-ipfs/commands"

	mh "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
	u "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
	cid "gx/ipfs/QmcEcrBAMrwMyhSjXt4yfyPpzgSuV8HLHavnfmiKCSRqZU/go-cid"
)

type BlockStat struct {
	Key  string
	Size int
}

func (bs BlockStat) String() string {
	return fmt.Sprintf("Key: %s\nSize: %d\n", bs.Key, bs.Size)
}

var BlockCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with raw IPFS blocks.",
		ShortDescription: `
'ipfs block' is a plumbing command used to manipulate raw IPFS blocks.
Reads from stdin or writes to stdout, and <key> is a base58 encoded
multihash.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"stat":   blockStatCmd,
		"get":    blockGetCmd,
		"put":    blockPutCmd,
		"rm":     blockRmCmd,
		"locate": blockLocateCmd,
	},
}

var blockStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print information of a raw IPFS block.",
		ShortDescription: `
'ipfs block stat' is a plumbing command for retrieving information
on raw IPFS blocks. It outputs the following to stdout:

	Key  - the base58 encoded multihash
	Size - the size of the block in bytes

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The base58 multihash of an existing block to stat.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		b, err := getBlockForKey(req, req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&BlockStat{
			Key:  b.Cid().String(),
			Size: len(b.RawData()),
		})
	},
	Type: BlockStat{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			bs := res.Output().(*BlockStat)
			return strings.NewReader(bs.String()), nil
		},
	},
}

var blockGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get a raw IPFS block.",
		ShortDescription: `
'ipfs block get' is a plumbing command for retrieving raw IPFS blocks.
It outputs to stdout, and <key> is a base58 encoded multihash.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The base58 multihash of an existing block to get.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		b, err := getBlockForKey(req, req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(bytes.NewReader(b.RawData()))
	},
}

var blockPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Store input as an IPFS block.",
		ShortDescription: `
'ipfs block put' is a plumbing command for storing raw IPFS blocks.
It reads from stdin, and <key> is a base58 encoded multihash.
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, false, "The data to be stored as an IPFS block.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("format", "f", "cid format for blocks to be created with.").Default("v0"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		file, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		data, err := ioutil.ReadAll(file)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = file.Close()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		format, _, _ := req.Option("format").String()
		var pref cid.Prefix
		pref.MhType = mh.SHA2_256
		pref.MhLength = -1
		pref.Version = 1
		switch format {
		case "cbor":
			pref.Codec = cid.CBOR
		case "json":
			pref.Codec = cid.JSON
		case "protobuf":
			pref.Codec = cid.Protobuf
		case "raw":
			pref.Codec = cid.Raw
		case "v0":
			pref.Version = 0
			pref.Codec = cid.Protobuf
		default:
			res.SetError(fmt.Errorf("unrecognized format: %s", format), cmds.ErrNormal)
			return
		}

		bcid, err := pref.Sum(data)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		b, err := blocks.NewBlockWithCid(data, bcid)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		log.Debugf("BlockPut key: '%q'", b.Cid())

		k, err := n.Blocks.AddBlock(b)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&BlockStat{
			Key:  k.String(),
			Size: len(data),
		})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			bs := res.Output().(*BlockStat)
			return strings.NewReader(bs.Key + "\n"), nil
		},
	},
	Type: BlockStat{},
}

func getBlockForKey(req cmds.Request, skey string) (blocks.Block, error) {
	if len(skey) == 0 {
		return nil, fmt.Errorf("zero length cid invalid")
	}

	n, err := req.InvocContext().GetNode()
	if err != nil {
		return nil, err
	}

	c, err := cid.Decode(skey)
	if err != nil {
		return nil, err
	}

	b, err := n.Blocks.GetBlock(req.Context(), c)
	if err != nil {
		return nil, err
	}

	log.Debugf("ipfs block: got block with key: %s", b.Cid())
	return b, nil
}

var blockRmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove IPFS block(s).",
		ShortDescription: `
'ipfs block rm' is a plumbing command for removing raw ipfs blocks.
It takes a list of base58 encoded multihashs to remove.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("hash", true, true, "Bash58 encoded multihash of block(s) to remove."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("force", "f", "Ignore nonexistent blocks.").Default(false),
		cmds.BoolOption("quiet", "q", "Write minimal output.").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		blockRmRun(req, res, "")
	},
	PostRun: func(req cmds.Request, res cmds.Response) {
		if res.Error() != nil {
			return
		}
		outChan, ok := res.Output().(<-chan interface{})
		if !ok {
			res.SetError(u.ErrCast(), cmds.ErrNormal)
			return
		}
		res.SetOutput(nil)

		err := util.ProcRmOutput(outChan, res.Stdout(), res.Stderr())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
		}
	},
	Type: util.RemovedBlock{},
}

func blockRmRun(req cmds.Request, res cmds.Response, prefix string) {
	n, err := req.InvocContext().GetNode()
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}
	hashes := req.Arguments()
	force, _, _ := req.Option("force").Bool()
	quiet, _, _ := req.Option("quiet").Bool()
	cids := make([]*cid.Cid, 0, len(hashes))
	for _, hash := range hashes {
		c, err := cid.Decode(hash)
		if err != nil {
			res.SetError(fmt.Errorf("invalid content id: %s (%s)", hash, err), cmds.ErrNormal)
			return
		}
		cids = append(cids, c)
	}
	outChan := make(chan interface{})
	err = util.RmBlocks(n.Blockstore, n.Pinning, outChan, cids, util.RmBlocksOpts{
		Quiet:  quiet,
		Force:  force,
		Prefix: prefix,
	})
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}
	res.SetOutput((<-chan interface{})(outChan))
}

type BlockLocateRes struct {
	Key string
	Res []bs.LocateInfo
}

var blockLocateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Locate an IPFS block.",
		ShortDescription: `
'ipfs block rm' is a plumbing command for locating which
sub-datastores block(s) are located in.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("hash", true, true, "Bash58 encoded multihash of block(s) to check."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Write minimal output.").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		hashes := req.Arguments()
		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))
		go func() {
			defer close(outChan)
			for _, hash := range hashes {
				key, err := cid.Decode(hash)
				if err != nil {
					panic(err) // FIXME
				}
				ret := n.Blockstore.Locate(key)
				outChan <- &BlockLocateRes{hash, ret}
			}
		}()
		return
	},
	PostRun: func(req cmds.Request, res cmds.Response) {
		if res.Error() != nil {
			return
		}
		quiet, _, _ := req.Option("quiet").Bool()
		outChan, ok := res.Output().(<-chan interface{})
		if !ok {
			res.SetError(u.ErrCast(), cmds.ErrNormal)
			return
		}
		res.SetOutput(nil)

		for out := range outChan {
			ret := out.(*BlockLocateRes)
			for _, inf := range ret.Res {
				if quiet && inf.Error == nil {
					fmt.Fprintf(res.Stdout(), "%s %s\n", ret.Key, inf.Prefix)
				} else if !quiet && inf.Error == nil {
					fmt.Fprintf(res.Stdout(), "%s %s found\n", ret.Key, inf.Prefix)
				} else if !quiet {
					fmt.Fprintf(res.Stdout(), "%s %s error  %s\n", ret.Key, inf.Prefix, inf.Error.Error())
				}
			}
		}
	},
	Type: BlockLocateRes{},
}
