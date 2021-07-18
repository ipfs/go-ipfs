package coreapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	gopath "path"
	"sync"
	"time"

	"github.com/ipfs/go-mfs"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

const defaultFilePerm = 0644
const defaultDirPerm = 0755

type MfsAPI CoreAPI

type fileNodeInfo struct {
	l mfs.NodeListing
}

func (i *fileNodeInfo) Name() string {
	return i.l.Name //TODO: check what is really returned here
}

func (i *fileNodeInfo) Size() int64 {
	return i.l.Size
}

func (i *fileNodeInfo) Mode() os.FileMode {
	return defaultFilePerm
}

func (i *fileNodeInfo) ModTime() time.Time {
	return time.Unix(0, 0)
}

func (i *fileNodeInfo) IsDir() bool {
	return mfs.NodeType(i.l.Type) == mfs.TDir
}

func (i *fileNodeInfo) Sys() interface{} {
	return i.l
}

func (api *MfsAPI) Create(ctx context.Context, path coreiface.MfsPath) (coreiface.File, error) {
	return api.OpenFile(ctx, path, os.O_CREATE|os.O_WRONLY, defaultFilePerm)
}

func (api *MfsAPI) Open(ctx context.Context, path coreiface.MfsPath) (coreiface.File, error) {
	return api.OpenFile(ctx, path, os.O_RDWR, defaultFilePerm)
}

type mfsFile struct {
	mfs.FileDescriptor
	lk sync.Mutex // Only needed for ReadAt, remove when it's in go-mfs
}

func (f *mfsFile) Read(p []byte) (n int, err error) {
	f.lk.Lock()
	defer f.lk.Unlock()
	return f.FileDescriptor.Read(p)
}

func (f *mfsFile) Write(p []byte) (n int, err error) {
	f.lk.Lock()
	defer f.lk.Unlock()
	return f.FileDescriptor.Write(p)
}

func (f *mfsFile) Seek(offset int64, whence int) (int64, error) {
	f.lk.Lock()
	defer f.lk.Unlock()
	return f.FileDescriptor.Seek(offset, whence)
}

func (f *mfsFile) Close() error {
	f.lk.Lock()
	defer f.lk.Unlock()
	return f.FileDescriptor.Close()
}

func (f *mfsFile) ReadAt(p []byte, off int64) (int, error) {
	// TODO: implement in MFS with less locking
	f.lk.Lock()
	defer f.lk.Unlock()

	cur, err := f.FileDescriptor.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	so, err := f.FileDescriptor.Seek(off, io.SeekStart)
	if err != nil {
		return 0, err
	}

	if so != off {
		return 0, errors.New("seek to wrong offset")
	}

	n, err := f.FileDescriptor.Read(p)
	if err != nil {
		return n, err
	}

	off, err = f.FileDescriptor.Seek(cur, io.SeekStart)
	if err != nil {
		return n, err
	}

	if cur != off {
		return n, errors.New("seek to wrong offset")
	}

	return n, nil
}

func (f *mfsFile) Name() coreiface.MfsPath {
	panic("implement me")
}

func (api *MfsAPI) OpenFile(ctx context.Context, path coreiface.MfsPath, flag int, perm os.FileMode) (coreiface.File, error) {
	fsn, err := mfs.Lookup(api.filesRoot, path.String())
	if err != nil {
		return nil, err
	}

	fn, ok := fsn.(*mfs.File)
	if !ok {
		return nil, fmt.Errorf("cannot open %s: not a file", path.String())
	}

	flags := mfs.Flags{
		Read:  flag&os.O_WRONLY == 0,
		Write: flag&(os.O_WRONLY|os.O_RDWR) != 0,
		Sync:  flag&os.O_SYNC != 0,
	}

	_, err = fn.Open(flags)
	if err != nil {
		return nil, err
	}

	return &mfsFile{}, nil
}

func (api *MfsAPI) Stat(ctx context.Context, path coreiface.MfsPath) (os.FileInfo, error) {
	fsn, err := mfs.Lookup(api.filesRoot, path.String())
	if err != nil {
		return nil, err
	}

	nd, err := fsn.GetNode()
	if err != nil {
		return nil, err
	}

	return &fileNodeInfo{
		l: mfs.NodeListing{
			Name: path.String(),
			Type: int(fsn.Type()),
			Size: -1, //TODO
			Hash: nd.Cid().String(),
		},
	}, nil
}

func (api *MfsAPI) Rename(ctx context.Context, oldpath, newpath coreiface.MfsPath) error {
	flush := false //TODO
	err := mfs.Mv(api.filesRoot, oldpath.String(), newpath.String())
	if err == nil && flush {
		err = mfs.FlushPath(api.filesRoot, "/")
	}
	return err
}

func (api *MfsAPI) Remove(ctx context.Context, path coreiface.MfsPath) error {
	if path.String() == "/" {
		return fmt.Errorf("cannot delete root")
	}

	dir, name := gopath.Split(path.String())
	parent, err := mfs.Lookup(api.filesRoot, dir)
	if err != nil {
		return fmt.Errorf("parent lookup: %s", err)
	}

	pdir, ok := parent.(*mfs.Directory)
	if !ok {
		return fmt.Errorf("no such file or directory: %s", path)
	}

	// TODO: force

	// get child node by name, when the node is corrupted and nonexistent,
	// it will return specific error.
	child, err := pdir.Child(name)
	if err != nil {
		return err
	}

	dashr := true // TODO: make into an option

	switch child.(type) {
	case *mfs.Directory:
		if !dashr {
			return fmt.Errorf("%s is a directory, use -r to remove directories", path)
		}
	}

	err = pdir.Unlink(name)
	if err != nil {
		return err
	}

	return pdir.Flush() //TODO: setting for flush
}

func (api *MfsAPI) ReadDir(ctx context.Context, path coreiface.MfsPath) ([]os.FileInfo, error) {
	fsn, err := mfs.Lookup(api.filesRoot, path.String())
	if err != nil {
		return nil, err
	}

	switch fsn := fsn.(type) {
	case *mfs.Directory:
		lst, err := fsn.List(ctx)
		if err != nil {
			return nil, err
		}

		out := make([]os.FileInfo, len(lst))
		for i, v := range lst {
			out[i] = &fileNodeInfo{
				l: v,
			}
		}

		return out, nil
	default:
		return nil, errors.New("readdir: unsupported node type")
	}
}

func (api *MfsAPI) MkdirAll(ctx context.Context, path coreiface.MfsPath, perm os.FileMode) error {
	return mfs.Mkdir(api.filesRoot, path.String(), mfs.MkdirOpts{
		Mkparents: true,
		Flush:     false,
		//CidBuilder: prefix, TODO
	})
}

func (api *MfsAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}

var _ os.FileInfo = &fileNodeInfo{}
