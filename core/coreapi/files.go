package coreapi

import (
	"context"
	"io"

	"github.com/ipfs/go-ipfs/core/fileshelpers"
	"github.com/ipfs/go-mfs"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

type FilesAPI CoreAPI

func (api *FilesAPI) Copy(ctx context.Context, src, dst string, opts *coreiface.FilesCopyOptions) error {
	return fileshelpers.Copy(ctx, api.filesRoot, (*CoreAPI)(api), src, dst, opts)
}

func (api *FilesAPI) Move(ctx context.Context, src, dst string, opts *coreiface.FilesMoveOptions) error {
	return fileshelpers.Move(ctx, api.filesRoot, src, dst, opts)
}

func (api *FilesAPI) List(ctx context.Context, path string, opts *coreiface.FilesListOptions) ([]mfs.NodeListing, error) {
	return fileshelpers.List(ctx, api.filesRoot, path, opts)
}

func (api *FilesAPI) Mkdir(ctx context.Context, path string, opts *coreiface.FilesMkdirOptions) error {
	return fileshelpers.Mkdir(ctx, api.filesRoot, path, opts)
}

func (api *FilesAPI) Read(ctx context.Context, path string, opts *coreiface.FilesReadOptions) (io.ReadCloser, error) {
	return fileshelpers.Read(ctx, api.filesRoot, path, opts)
}

func (api *FilesAPI) Remove(ctx context.Context, path string, opts *coreiface.FilesRemoveOptions) error {
	return fileshelpers.Remove(ctx, api.filesRoot, path, opts)
}

func (api *FilesAPI) Stat(ctx context.Context, path string, opts *coreiface.FilesStatOptions) (*coreiface.FileInfo, error) {
	return fileshelpers.Stat(
		ctx,
		api.filesRoot,
		(*CoreAPI)(api),
		api.blockstore,
		api.dag,
		path,
		opts,
	)
}

func (api *FilesAPI) Write(ctx context.Context, path string, r io.Reader, opts *coreiface.FilesWriteOptions) error {
	return fileshelpers.Write(ctx, api.filesRoot, path, r, opts)
}

func (api *FilesAPI) ChangeCid(ctx context.Context, path string, opts *coreiface.FilesChangeCidOptions) error {
	return fileshelpers.ChangeCid(ctx, api.filesRoot, path, opts)
}

func (api *FilesAPI) Flush(ctx context.Context, path string) error {
	_, err := mfs.FlushPath(ctx, api.filesRoot, path)
	return err
}
