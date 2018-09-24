package filestore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	pb "github.com/ipfs/go-ipfs/filestore/pb"

	posinfo "gx/ipfs/QmPG32VXR5jmpo9q8R9FNdR4Ae97Ky9CiZE6SctJLUB79H/go-ipfs-posinfo"
	dshelp "gx/ipfs/QmPQ7bVbZAbGaJkBVJeTkkKXvLLZeN9CLWTf5fzUQ8yeWs/go-ipfs-ds-help"
	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	blocks "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
	ds "gx/ipfs/QmSpg1CvpXQQow5ernt1gNBXaXV6yxyNqi7XoeerWfzB5w/go-datastore"
	dsns "gx/ipfs/QmSpg1CvpXQQow5ernt1gNBXaXV6yxyNqi7XoeerWfzB5w/go-datastore/namespace"
	dsq "gx/ipfs/QmSpg1CvpXQQow5ernt1gNBXaXV6yxyNqi7XoeerWfzB5w/go-datastore/query"
	proto "gx/ipfs/QmdxUuburamoF6zF9qjeQC4WYcWGbWuRmdLacMEsW8ioD8/gogo-protobuf/proto"
	blockstore "gx/ipfs/QmegPGspn3RpTMQ23Fd3GVVMopo1zsEMurudbFMZ5UXBLH/go-ipfs-blockstore"
)

// FilestorePrefix identifies the key prefix for FileManager blocks.
var FilestorePrefix = ds.NewKey("filestore")

// FileManager is a blockstore implementation which stores special
// blocks FilestoreNode type. These nodes only contain a reference
// to the actual location of the block data in the filesystem
// (a path and an offset).
type FileManager struct {
	AllowFiles bool
	AllowUrls  bool
	ds         ds.Batching
	root       string
}

// CorruptReferenceError implements the error interface.
// It is used to indicate that the block contents pointed
// by the referencing blocks cannot be retrieved (i.e. the
// file is not found, or the data changed as it was being read).
type CorruptReferenceError struct {
	Code Status
	Err  error
}

// Error() returns the error message in the CorruptReferenceError
// as a string.
func (c CorruptReferenceError) Error() string {
	return c.Err.Error()
}

// NewFileManager initializes a new file manager with the given
// datastore and root. All FilestoreNodes paths are relative to the
// root path given here, which is prepended for any operations.
func NewFileManager(ds ds.Batching, root string) *FileManager {
	return &FileManager{ds: dsns.Wrap(ds, FilestorePrefix), root: root}
}

// AllKeysChan returns a channel from which to read the keys stored in
// the FileManager. If the given context is cancelled the channel will be
// closed.
func (f *FileManager) AllKeysChan(ctx context.Context) (<-chan mh.Multihash, error) {
	q := dsq.Query{KeysOnly: true}

	res, err := f.ds.Query(q)
	if err != nil {
		return nil, err
	}

	out := make(chan mh.Multihash, dsq.KeysOnlyBufSize)
	go func() {
		defer close(out)
		for {
			v, ok := res.NextSync()
			if !ok {
				return
			}

			k := ds.RawKey(v.Key)
			h, err := dshelp.DsKeyToMultihash(k)
			if err != nil {
				log.Errorf("decoding cid from filestore: %s", err)
				continue
			}

			select {
			case out <- h:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// Delete deletes the reference-block from the underlying
// datastore. It does not touch the referenced data.
func (f *FileManager) Delete(k mh.Multihash) error {
	err := f.ds.Delete(dshelp.MultihashToDsKey(k))
	if err == ds.ErrNotFound {
		return blockstore.ErrNotFound
	}
	return err
}

// Get reads a block from the datastore. Reading a block
// is done in two steps: the first step retrieves the reference
// block from the datastore. The second step uses the stored
// path and offsets to read the raw block data directly from disk.
func (f *FileManager) Get(c cid.Cid) (blocks.Block, error) {
	dobj, err := f.getDataObj(c.Hash())
	if err != nil {
		return nil, err
	}
	out, err := f.readDataObj(c.Hash(), dobj)
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(out, c)
}

// GetSize gets the size of the block from the datastore.
//
// This method may successfully return the size even if returning the block
// would fail because the associated file is no longer available.
func (f *FileManager) GetSize(c mh.Multihash) (int, error) {
	dobj, err := f.getDataObj(c)
	if err != nil {
		return -1, err
	}
	return int(dobj.GetSize_()), nil
}

func (f *FileManager) readDataObj(c mh.Multihash, d *pb.DataObj) ([]byte, error) {
	if IsURL(d.GetFilePath()) {
		return f.readURLDataObj(c, d)
	}
	return f.readFileDataObj(c, d)
}

func (f *FileManager) getDataObj(k mh.Multihash) (*pb.DataObj, error) {
	o, err := f.ds.Get(dshelp.MultihashToDsKey(k))
	switch err {
	case ds.ErrNotFound:
		return nil, blockstore.ErrNotFound
	default:
		return nil, err
	case nil:
		//
	}

	return unmarshalDataObj(o)
}

func unmarshalDataObj(data []byte) (*pb.DataObj, error) {
	var dobj pb.DataObj
	if err := proto.Unmarshal(data, &dobj); err != nil {
		return nil, err
	}

	return &dobj, nil
}

func (f *FileManager) readFileDataObj(k mh.Multihash, d *pb.DataObj) ([]byte, error) {
	if !f.AllowFiles {
		return nil, ErrFilestoreNotEnabled
	}

	p := filepath.FromSlash(d.GetFilePath())
	abspath := filepath.Join(f.root, p)

	fi, err := os.Open(abspath)
	if os.IsNotExist(err) {
		return nil, &CorruptReferenceError{StatusFileNotFound, err}
	} else if err != nil {
		return nil, &CorruptReferenceError{StatusFileError, err}
	}
	defer fi.Close()

	_, err = fi.Seek(int64(d.GetOffset()), io.SeekStart)
	if err != nil {
		return nil, &CorruptReferenceError{StatusFileError, err}
	}

	outbuf := make([]byte, d.GetSize_())
	_, err = io.ReadFull(fi, outbuf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return nil, &CorruptReferenceError{StatusFileChanged, err}
	} else if err != nil {
		return nil, &CorruptReferenceError{StatusFileError, err}
	}

	dmh, err := mh.Decode(k)
	if err != nil {
		return nil, err
	}
	outhash, err := mh.Sum(outbuf, dmh.Code, dmh.Length)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(k, outhash) {
		return nil, &CorruptReferenceError{StatusFileChanged,
			fmt.Errorf("data in file did not match. %s offset %d", d.GetFilePath(), d.GetOffset())}
	}

	return outbuf, nil
}

// reads and verifies the block from URL
func (f *FileManager) readURLDataObj(k mh.Multihash, d *pb.DataObj) ([]byte, error) {
	if !f.AllowUrls {
		return nil, ErrUrlstoreNotEnabled
	}

	req, err := http.NewRequest("GET", d.GetFilePath(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", d.GetOffset(), d.GetOffset()+d.GetSize_()-1))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, &CorruptReferenceError{StatusFileError, err}
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		return nil, &CorruptReferenceError{StatusFileError,
			fmt.Errorf("expected HTTP 200 or 206 got %d", res.StatusCode)}
	}

	outbuf := make([]byte, d.GetSize_())
	_, err = io.ReadFull(res.Body, outbuf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return nil, &CorruptReferenceError{StatusFileChanged, err}
	} else if err != nil {
		return nil, &CorruptReferenceError{StatusFileError, err}
	}
	res.Body.Close()

	dmh, err := mh.Decode(k)
	if err != nil {
		return nil, err
	}
	outhash, err := mh.Sum(outbuf, dmh.Code, dmh.Length)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(k, outhash) {
		return nil, &CorruptReferenceError{StatusFileChanged,
			fmt.Errorf("data in file did not match. %s offset %d", d.GetFilePath(), d.GetOffset())}
	}

	return outbuf, nil
}

// Has returns if the FileManager is storing a block reference. It does not
// validate the data, nor checks if the reference is valid.
func (f *FileManager) Has(k mh.Multihash) (bool, error) {
	// NOTE: interesting thing to consider. Has doesnt validate the data.
	// So the data on disk could be invalid, and we could think we have it.
	dsk := dshelp.MultihashToDsKey(k)
	return f.ds.Has(dsk)
}

type putter interface {
	Put(ds.Key, []byte) error
}

// Put adds a new reference block to the FileManager. It does not check
// that the reference is valid.
func (f *FileManager) Put(b *posinfo.FilestoreNode) error {
	return f.putTo(b, f.ds)
}

func (f *FileManager) putTo(b *posinfo.FilestoreNode, to putter) error {
	var dobj pb.DataObj

	if IsURL(b.PosInfo.FullPath) {
		if !f.AllowUrls {
			return ErrUrlstoreNotEnabled
		}
		dobj.FilePath = b.PosInfo.FullPath
	} else {
		if !f.AllowFiles {
			return ErrFilestoreNotEnabled
		}
		if !filepath.HasPrefix(b.PosInfo.FullPath, f.root) {
			return fmt.Errorf("cannot add filestore references outside ipfs root (%s)", f.root)
		}

		p, err := filepath.Rel(f.root, b.PosInfo.FullPath)
		if err != nil {
			return err
		}

		dobj.FilePath = filepath.ToSlash(p)
	}
	dobj.Offset = b.PosInfo.Offset
	dobj.Size_ = uint64(len(b.RawData()))

	data, err := proto.Marshal(&dobj)
	if err != nil {
		return err
	}

	return to.Put(dshelp.CidToDsKey(b.Cid()), data)
}

// PutMany is like Put() but takes a slice of blocks instead,
// allowing it to create a batch transaction.
func (f *FileManager) PutMany(bs []*posinfo.FilestoreNode) error {
	batch, err := f.ds.Batch()
	if err != nil {
		return err
	}

	for _, b := range bs {
		if err := f.putTo(b, batch); err != nil {
			return err
		}
	}

	return batch.Commit()
}

// IsURL returns true if the string represents a valid URL that the
// urlstore can handle.  More specifically it returns true if a string
// begins with 'http://' or 'https://'.
func IsURL(str string) bool {
	return (len(str) > 7 && str[0] == 'h' && str[1] == 't' && str[2] == 't' && str[3] == 'p') &&
		((len(str) > 8 && str[4] == 's' && str[5] == ':' && str[6] == '/' && str[7] == '/') ||
			(str[4] == ':' && str[5] == '/' && str[6] == '/'))
}
