package fileshelpers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	gopath "path"
	"strings"

	bservice "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	dag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-mfs"
	ft "github.com/ipfs/go-unixfs"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

var log = logging.Logger("core/fileshelpers")
var defaultMoveOptions = &iface.FilesMoveOptions{Flush: true}

func Move(ctx context.Context, root *mfs.Root, src, dst string, opts *iface.FilesMoveOptions) error {
	if opts == nil {
		opts = defaultMoveOptions
	}

	src, err := checkPath(src)
	if err != nil {
		return err
	}
	dst, err = checkPath(dst)
	if err != nil {
		return err
	}

	err = mfs.Mv(root, src, dst)
	if err == nil && opts.Flush {
		_, err = mfs.FlushPath(ctx, root, "/")
	}
	return err
}

var defaultCopyOptions = &iface.FilesCopyOptions{Flush: true}

func Copy(ctx context.Context, root *mfs.Root, api iface.CoreAPI, src, dst string, opts *iface.FilesCopyOptions) error {
	if opts == nil {
		opts = defaultCopyOptions
	}

	src, err := checkPath(src)
	if err != nil {
		return err
	}

	dst, err = checkPath(dst)
	if err != nil {
		return err
	}

	if dst[len(dst)-1] == '/' {
		dst += gopath.Base(src)
	}

	node, err := getNodeFromPath(ctx, root, api, src)
	if err != nil {
		return fmt.Errorf("cp: cannot get node from path %s: %s", src, err)
	}

	err = mfs.PutNode(root, dst, node)
	if err != nil {
		return fmt.Errorf("cp: cannot put node in path %s: %s", dst, err)
	}

	if opts.Flush {
		_, err = mfs.FlushPath(ctx, root, dst)
		if err != nil {
			return fmt.Errorf("cp: cannot flush the created file %s: %s", dst, err)
		}
	}

	return nil
}

func Remove(ctx context.Context, root *mfs.Root, path string, opts *iface.FilesRemoveOptions) error {
	if opts == nil {
		opts = &iface.FilesRemoveOptions{}
	}

	path, err := checkPath(path)
	if err != nil {
		return err
	}

	if path == "/" {
		return fmt.Errorf("cannot delete root")
	}

	// 'rm a/b/c/' will fail unless we trim the slash at the end
	if path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	dir, name := gopath.Split(path)

	pdir, err := getParentDir(root, dir)
	if err != nil {
		if opts.Force && err == os.ErrNotExist {
			return nil
		}
		return fmt.Errorf("parent lookup: %s", err)
	}

	// if 'force' specified, it will remove anything else,
	// including file, directory, corrupted node, etc
	if opts.Force {
		err := pdir.Unlink(name)
		if err != nil {
			if err == os.ErrNotExist {
				return nil
			}
			return err
		}
		return pdir.Flush()
	}

	// get child node by name, when the node is corrupted and nonexistent,
	// it will return specific error.
	child, err := pdir.Child(name)
	if err != nil {
		return err
	}

	switch child.(type) {
	case *mfs.Directory:
		if !opts.Recursive {
			return fmt.Errorf("%s is a directory, use -r to remove directories", path)
		}
	}

	err = pdir.Unlink(name)
	if err != nil {
		return err
	}

	return pdir.Flush()
}

func List(ctx context.Context, root *mfs.Root, path string, opts *iface.FilesListOptions) ([]mfs.NodeListing, error) {
	if opts == nil {
		opts = &iface.FilesListOptions{
			CidEncoder: cidenc.Default(),
		}
	}

	path, err := checkPath(path)
	if err != nil {
		return nil, err
	}

	fsn, err := mfs.Lookup(root, path)
	if err != nil {
		return nil, err
	}

	switch fsn := fsn.(type) {
	case *mfs.Directory:
		if !opts.Long {
			var output []mfs.NodeListing
			names, err := fsn.ListNames(ctx)
			if err != nil {
				return nil, err
			}

			for _, name := range names {
				output = append(output, mfs.NodeListing{
					Name: name,
				})
			}
			return output, nil
		}
		return fsn.List(ctx)
	case *mfs.File:
		_, name := gopath.Split(path)
		out := []mfs.NodeListing{{Name: name}}
		if opts.Long {
			out[0].Type = int(fsn.Type())

			size, err := fsn.Size()
			if err != nil {
				return nil, err
			}
			out[0].Size = size

			nd, err := fsn.GetNode()
			if err != nil {
				return nil, err
			}
			out[0].Hash = opts.CidEncoder.Encode(nd.Cid())
		}
		return out, nil
	default:
		return nil, errors.New("unrecognized type")
	}
}

func Write(ctx context.Context, root *mfs.Root, path string, r io.Reader, opts *iface.FilesWriteOptions) (retErr error) {
	if opts == nil {
		opts = &iface.FilesWriteOptions{}
	}

	path, err := checkPath(path)
	if err != nil {
		return err
	}

	if opts.Offset < 0 {
		return fmt.Errorf("cannot have negative write offset")
	}

	if opts.MakeParents {
		err := ensureContainingDirectoryExists(root, path, opts.CidBuilder)
		if err != nil {
			return err
		}
	}

	fi, err := getFileHandle(root, path, opts.Create, opts.CidBuilder)
	if err != nil {
		return err
	}

	if opts.RawLeavesOverride {
		fi.RawLeaves = opts.RawLeaves
	}

	wfd, err := fi.Open(mfs.Flags{Write: true, Sync: opts.Flush})
	if err != nil {
		return err
	}

	defer func() {
		err := wfd.Close()
		if err != nil {
			if retErr == nil {
				retErr = err
			} else {
				log.Error("files: error closing file mfs file descriptor", err)
			}
		}
	}()

	if opts.Truncate {
		if err := wfd.Truncate(0); err != nil {
			return err
		}
	}

	if opts.Count < 0 {
		return fmt.Errorf("cannot have negative byte count")
	}

	_, err = wfd.Seek(int64(opts.Offset), io.SeekStart)
	if err != nil {
		return err
	}

	if opts.Count > 0 {
		r = io.LimitReader(r, int64(opts.Count))
	}

	_, err = io.Copy(wfd, r)
	return err
}

func ChangeCid(ctx context.Context, root *mfs.Root, path string, opts *iface.FilesChangeCidOptions) error {
	if opts == nil {
		opts = &iface.FilesChangeCidOptions{}
	}

	if opts.CidBuilder == nil {
		return nil
	}

	nd, err := mfs.Lookup(root, path)
	if err != nil {
		return err
	}

	switch n := nd.(type) {
	case *mfs.Directory:
		n.SetCidBuilder(opts.CidBuilder)
	default:
		return fmt.Errorf("can only update directories")
	}

	if opts.Flush {
		_, err = mfs.FlushPath(ctx, root, path)
	}

	return err
}

func Mkdir(ctx context.Context, root *mfs.Root, path string, opts *iface.FilesMkdirOptions) error {
	if opts == nil {
		opts = &iface.FilesMkdirOptions{}
	}

	dirtomake, err := checkPath(path)
	if err != nil {
		return err
	}

	err = mfs.Mkdir(root, dirtomake, mfs.MkdirOpts{
		Mkparents:  opts.MakeParents,
		Flush:      opts.Flush,
		CidBuilder: opts.CidBuilder,
	})

	return err
}

func ensureContainingDirectoryExists(r *mfs.Root, path string, builder cid.Builder) error {
	dirtomake := gopath.Dir(path)

	if dirtomake == "/" {
		return nil
	}

	return mfs.Mkdir(r, dirtomake, mfs.MkdirOpts{
		Mkparents:  true,
		CidBuilder: builder,
	})
}

func getFileHandle(r *mfs.Root, path string, create bool, builder cid.Builder) (*mfs.File, error) {
	target, err := mfs.Lookup(r, path)
	switch err {
	case nil:
		fi, ok := target.(*mfs.File)
		if !ok {
			return nil, fmt.Errorf("%s was not a file", path)
		}
		return fi, nil

	case os.ErrNotExist:
		if !create {
			return nil, err
		}

		// if create is specified and the file doesnt exist, we create the file
		dirname, fname := gopath.Split(path)
		pdir, err := getParentDir(r, dirname)
		if err != nil {
			return nil, err
		}

		if builder == nil {
			builder = pdir.GetCidBuilder()
		}

		nd := dag.NodeWithData(ft.FilePBData(nil, 0))
		nd.SetCidBuilder(builder)
		err = pdir.AddChild(fname, nd)
		if err != nil {
			return nil, err
		}

		fsn, err := pdir.Child(fname)
		if err != nil {
			return nil, err
		}

		fi, ok := fsn.(*mfs.File)
		if !ok {
			return nil, errors.New("expected *mfs.File, didnt get it. This is likely a race condition")
		}
		return fi, nil

	default:
		return nil, err
	}
}

func Read(ctx context.Context, root *mfs.Root, path string, opts *iface.FilesReadOptions) (io.ReadCloser, error) {
	if opts == nil {
		opts = &iface.FilesReadOptions{}
	}

	path, err := checkPath(path)
	if err != nil {
		return nil, err
	}

	fsn, err := mfs.Lookup(root, path)
	if err != nil {
		return nil, err
	}

	fi, ok := fsn.(*mfs.File)
	if !ok {
		return nil, fmt.Errorf("%s was not a file", path)
	}

	rfd, err := fi.Open(mfs.Flags{Read: true})
	if err != nil {
		return nil, err
	}

	if opts.Offset < 0 {
		return nil, fmt.Errorf("cannot specify negative offset")
	}

	filen, err := rfd.Size()
	if err != nil {
		return nil, err
	}

	if int64(opts.Offset) > filen {
		return nil, fmt.Errorf("offset was past end of file (%d > %d)", opts.Offset, filen)
	}

	_, err = rfd.Seek(int64(opts.Offset), io.SeekStart)
	if err != nil {
		return nil, err
	}

	var r io.ReadCloser = &contextReaderWrapper{
		R:   rfd,
		ctx: ctx,
	}

	if opts.Count < 0 {
		return nil, fmt.Errorf("cannot specify negative 'count'")
	}

	if opts.Count > 0 {
		r = &limitReaderCloserWrapper{
			Reader: io.LimitReader(r, int64(opts.Count)),
			F:      rfd,
		}
	}

	return r, nil
}

type contextReadCloser interface {
	CtxReadFull(context.Context, []byte) (int, error)
	Close() error
}

type contextReaderWrapper struct {
	R   contextReadCloser
	ctx context.Context
}

func (crw *contextReaderWrapper) Read(b []byte) (int, error) {
	return crw.R.CtxReadFull(crw.ctx, b)
}

func (crw *contextReaderWrapper) Close() error {
	return crw.R.Close()
}

type limitReaderCloserWrapper struct {
	io.Reader
	F mfs.FileDescriptor
}

func (lrcw *limitReaderCloserWrapper) Close() error {
	return lrcw.F.Close()
}

func Stat(ctx context.Context, root *mfs.Root, api iface.CoreAPI, bs blockstore.GCBlockstore, ds ipld.DAGService, path string, opts *iface.FilesStatOptions) (*iface.FileInfo, error) {
	if opts == nil {
		opts = &iface.FilesStatOptions{
			CidEncoder: cidenc.Default(),
		}
	}

	path, err := checkPath(path)
	if err != nil {
		return nil, err
	}

	var dagserv ipld.DAGService
	if opts.WithLocality {
		// an offline DAGService will not fetch from the network
		dagserv = dag.NewDAGService(bservice.New(
			bs,
			offline.Exchange(bs),
		))
	} else {
		dagserv = ds
	}

	nd, err := getNodeFromPath(ctx, root, api, path)
	if err != nil {
		return nil, err
	}

	o, err := statNode(nd, opts.CidEncoder)
	if err != nil {
		return nil, err
	}

	if !opts.WithLocality {
		return o, nil
	}

	local, sizeLocal, err := walkBlock(ctx, dagserv, nd)
	if err != nil {
		return nil, err
	}

	o.WithLocality = true
	o.Local = local
	o.SizeLocal = sizeLocal
	return o, nil
}

func walkBlock(ctx context.Context, dagserv ipld.DAGService, nd ipld.Node) (bool, uint64, error) {
	// Start with the block data size
	sizeLocal := uint64(len(nd.RawData()))

	local := true

	for _, link := range nd.Links() {
		child, err := dagserv.Get(ctx, link.Cid)

		if err == ipld.ErrNotFound {
			local = false
			continue
		}

		if err != nil {
			return local, sizeLocal, err
		}

		childLocal, childLocalSize, err := walkBlock(ctx, dagserv, child)

		if err != nil {
			return local, sizeLocal, err
		}

		// Recursively add the child size
		local = local && childLocal
		sizeLocal += childLocalSize
	}

	return local, sizeLocal, nil
}

func getNodeFromPath(ctx context.Context, root *mfs.Root, api iface.CoreAPI, p string) (ipld.Node, error) {
	switch {
	case strings.HasPrefix(p, "/ipfs/"):
		return api.ResolveNode(ctx, path.New(p))
	default:
		fsn, err := mfs.Lookup(root, p)
		if err != nil {
			return nil, err
		}

		return fsn.GetNode()
	}
}

func getParentDir(root *mfs.Root, dir string) (*mfs.Directory, error) {
	parent, err := mfs.Lookup(root, dir)
	if err != nil {
		return nil, err
	}

	pdir, ok := parent.(*mfs.Directory)
	if !ok {
		return nil, errors.New("expected *mfs.Directory, didnt get it. This is likely a race condition")
	}
	return pdir, nil
}

func checkPath(p string) (string, error) {
	if len(p) == 0 {
		return "", fmt.Errorf("paths must not be empty")
	}

	if p[0] != '/' {
		return "", fmt.Errorf("paths must start with a leading slash")
	}

	cleaned := gopath.Clean(p)
	if p[len(p)-1] == '/' && p != "/" {
		cleaned += "/"
	}
	return cleaned, nil
}

func statNode(nd ipld.Node, enc cidenc.Encoder) (*iface.FileInfo, error) {
	c := nd.Cid()

	cumulsize, err := nd.Size()
	if err != nil {
		return nil, err
	}

	switch n := nd.(type) {
	case *dag.ProtoNode:
		d, err := ft.FSNodeFromBytes(n.Data())
		if err != nil {
			return nil, err
		}

		var ndtype string
		switch d.Type() {
		case ft.TDirectory, ft.THAMTShard:
			ndtype = "directory"
		case ft.TFile, ft.TMetadata, ft.TRaw:
			ndtype = "file"
		default:
			return nil, fmt.Errorf("unrecognized node type: %s", d.Type())
		}

		return &iface.FileInfo{
			Hash:           enc.Encode(c),
			Blocks:         len(nd.Links()),
			Size:           d.FileSize(),
			CumulativeSize: cumulsize,
			Type:           ndtype,
		}, nil
	case *dag.RawNode:
		return &iface.FileInfo{
			Hash:           enc.Encode(c),
			Blocks:         0,
			Size:           cumulsize,
			CumulativeSize: cumulsize,
			Type:           "file",
		}, nil
	default:
		return nil, fmt.Errorf("not unixfs node (proto or raw)")
	}
}
