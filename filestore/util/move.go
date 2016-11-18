package filestore_util

import (
	errs "errors"
	"io"
	"os"
	"path/filepath"

	"github.com/ipfs/go-ipfs/core"
	. "github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/ipfs/go-ipfs/unixfs"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	dag "github.com/ipfs/go-ipfs/merkledag"
	//dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
)

// type fileNodes map[bk.Key]struct{}

// func (m fileNodes) have(key bk.Key) bool {
//	_, ok := m[key]
//	return ok
//}

//func (m fileNodes) add(key bk.Key) {
//	m[key] = struct{}{}
//}

// func extractFiles(key bk.Key, fs *Datastore, bs b.Blockservice, res *fileNodes) error {
// 	n, dataObj, status := getNode(key.DsKey(), key, fs, bs)
// 	if AnError(status) {
// 		return fmt.Errorf("Error when retrieving key: %s.", key)
// 	}
// 	if dataObj != nil {
// 		// already in filestore
// 		return nil
// 	}
// 	fsnode, err := unixfs.FromBytes(n.Data)
// 	if err != nil {
// 		return err
// 	}
// 	switch *fsnode.Type {
// 	case unixfs.TRaw:
// 	case unixfs.TFile:
// 		res.add(key)
// 	case unixfs.TDirectory:
// 		for _, link := range n.Links {
// 			err := extractFiles(bk.Key(link.Hash), fs, bs, res)
// 			if err != nil {
// 				return err
// 			}
// 		}
// 	default:
// 	}
// 	return nil
// }

func ConvertToFile(node *core.IpfsNode, k *cid.Cid, path string) error {
	config, _ := node.Repo.Config()
	if !node.LocalMode() && (config == nil || !config.Filestore.APIServerSidePaths) {
		return errs.New("Daemon is running and server side paths are not enabled.")
	}
	if !filepath.IsAbs(path) {
		return errs.New("absolute path required")
	}
	wtr, err := os.Create(path)
	if err != nil {
		return err
	}
	fs, ok := node.Repo.DirectMount(fsrepo.FilestoreMount).(*Datastore)
	if !ok {
		return errs.New("Could not extract filestore.")
	}
	p := params{node.Blockstore, fs, path, wtr}
	_, err = p.convertToFile(k, true, 0)
	return err
}

type params struct {
	bs   b.Blockstore
	fs   *Datastore
	path string
	out  io.Writer
}

func (p *params) convertToFile(k *cid.Cid, root bool, offset uint64) (uint64, error) {
	block, err := p.bs.Get(k)
	if err != nil {
		return 0, err
	}
	altData, fsInfo, err := Reconstruct(block.RawData(), nil, 0)
	if err != nil {
		return 0, err
	}
	if fsInfo.Type != unixfs.TRaw && fsInfo.Type != unixfs.TFile {
		return 0, errs.New("Not a file")
	}
	dataObj := &DataObj{
		FilePath: p.path,
		Offset:   offset,
		Size:     fsInfo.FileSize,
	}
	if root {
		dataObj.Flags = WholeFile
	}
	if len(fsInfo.Data) > 0 {
		_, err := p.out.Write(fsInfo.Data)
		if err != nil {
			return 0, err
		}
		dataObj.Flags |= NoBlockData
		dataObj.Data = altData
		return 0, errs.New("Unimplemeted")
		//p.fs.Update(dshelp.CidToDsKey(k).Bytes(), nil, dataObj)
	} else {
		dataObj.Flags |= Internal
		dataObj.Data = block.RawData()
		return 0, errs.New("Unimplemeted")
		//p.fs.Update(dshelp.CidToDsKey(k).Bytes(), nil, dataObj)
		n, err := dag.DecodeProtobuf(block.RawData())
		if err != nil {
			return 0, err
		}
		for _, link := range n.Links() {
			size, err := p.convertToFile(link.Cid, false, offset)
			if err != nil {
				return 0, err
			}
			offset += size
		}
	}
	return fsInfo.FileSize, nil
}
