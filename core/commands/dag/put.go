package dagcmd

import (
	"bytes"
	"fmt"
	"strconv"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	ipldlegacy "github.com/ipfs/go-ipld-legacy"
	dagpb "github.com/ipld/go-codec-dagpb"
	"github.com/ipld/go-ipld-prime/multicodec"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal"

	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	mc "github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"

	// Expected minimal set of available format/ienc codecs.
	_ "github.com/ipld/go-codec-dagpb"
	_ "github.com/ipld/go-ipld-prime/codec/cbor"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	_ "github.com/ipld/go-ipld-prime/codec/dagjson"
	_ "github.com/ipld/go-ipld-prime/codec/json"
	_ "github.com/ipld/go-ipld-prime/codec/raw"
)

func dagPut(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}

	ienc, _ := req.Options["input-enc"].(string)
	format, _ := req.Options["format"].(string)
	hash, _ := req.Options["hash"].(string)
	dopin, _ := req.Options["pin"].(bool)
	wrappb, _ := req.Options["wrap-protobuf"].(bool)

	// mhType tells inputParser which hash should be used. Default otherwise is sha256
	mhType := uint64(mh.SHA2_256)

	icodec, ok := mc.Of(ienc)
	if !ok {
		n, err := strconv.Atoi(ienc)
		if err != nil {
			return fmt.Errorf("%s is not a valid codec name", ienc)
		}
		icodec = mc.Code(n)
	}
	fcodec, ok := mc.Of(format)
	if !ok {
		n, err := strconv.Atoi(format)
		if err != nil {
			return fmt.Errorf("%s is not a valid codec name", format)
		}
		fcodec = mc.Code(n)
	}

	cidPrefix := cid.Prefix{
		Version:  1,
		Codec:    uint64(fcodec),
		MhType:   mhType,
		MhLength: -1,
	}

	if hash != "" {
		var ok bool
		mhType, ok = mh.Names[hash]
		if !ok {
			return fmt.Errorf("%s in not a valid multihash name", hash)
		}
		cidPrefix.MhType = mhType
	}

	decoder, err := multicodec.LookupDecoder(uint64(icodec))
	if err != nil {
		return err
	}
	encoder, err := multicodec.LookupEncoder(uint64(fcodec))
	if err != nil {
		return err
	}

	var adder ipld.NodeAdder = api.Dag()
	if dopin {
		adder = api.Dag().Pinning()
	}
	b := ipld.NewBatch(req.Context, adder)

	it := req.Files.Entries()
	for it.Next() {
		file := files.FileFromEntry(it)
		if file == nil {
			return fmt.Errorf("expected a regular file")
		}

		node := basicnode.Prototype.Any.NewBuilder()
		if err := decoder(node, file); err != nil {
			return err
		}
		n := node.Build()

		bd := bytes.NewBuffer([]byte{})
		if err := encoder(n, bd); err != nil {
			return err
		}

		if wrappb {
			builder := dagpb.Type.PBNode.NewBuilder()
			pbm, err := builder.BeginMap(2)
			if err != nil {
				return err
			}
			data, err := pbm.AssembleEntry("Data")
			if err != nil {
				return err
			}
			if err := data.AssignBytes(bd.Bytes()); err != nil {
				return err
			}
			links, err := pbm.AssembleEntry("Links")
			if err != nil {
				return err
			}
			linkSlice, err := traversal.SelectLinks(n)
			if err != nil {
				return err
			}
			pbl, err := links.BeginList(int64(len(linkSlice)))
			if err != nil {
				return err
			}
			for _, l := range linkSlice {
				if err := pbl.AssembleValue().AssignLink(l); err != nil {
					return err
				}
			}
			if err := pbl.Finish(); err != nil {
				return err
			}
			if err := pbm.Finish(); err != nil {
				return err
			}
			n = builder.Build()
			bd = bytes.NewBuffer([]byte{})
			if err := dagpb.Encode(n, bd); err != nil {
				return err
			}
		}

		blockCid, err := cidPrefix.Sum(bd.Bytes())
		if err != nil {
			return err
		}
		blk, err := blocks.NewBlockWithCid(bd.Bytes(), blockCid)
		if err != nil {
			return err
		}
		ln := ipldlegacy.LegacyNode{
			Block: blk,
			Node:  n,
		}

		if err := b.Add(req.Context, &ln); err != nil {
			return err
		}
		/*
			for _, nd := range nds {
				err := b.Add(req.Context, nd)
				if err != nil {
					return err
				}
			}
		*/

		cid := ln.Cid()
		if err := res.Emit(&OutputObject{Cid: cid}); err != nil {
			return err
		}
	}
	if it.Err() != nil {
		return it.Err()
	}

	if err := b.Commit(); err != nil {
		return err
	}

	return nil
}
