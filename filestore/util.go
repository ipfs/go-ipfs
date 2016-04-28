package filestore

import (
	"fmt"
	"io"
	"os"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/query"
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
)

const (
	StatusOk      = 1
	StatusMissing = 2
	StatusInvalid = 3
	StatusError   = 4
)

func statusStr(status int) string {
	switch status {
	case 0:
		return ""
	case 1:
		return "ok      "
	case 2:
		return "missing "
	case 3:
		return "invalid "
	case 4:
		return "error   "
	default:
		return "??      "
	}
}

type ListRes struct {
	Key []byte
	DataObj
	Status int
}

func (r *ListRes) Format() string {
	mhash := b58.Encode(r.Key)
	return fmt.Sprintf("%s%s %s\n", statusStr(r.Status), mhash, r.DataObj.Format())
}

func list(d *Datastore, out chan<- *ListRes, verify bool) error {
	qr, err := d.Query(query.Query{KeysOnly: true})
	if err != nil {
		return err
	}
	for r := range qr.Next() {
		if r.Error != nil {
			return r.Error
		}
		key := ds.NewKey(r.Key)
		val, _ := d.GetDirect(key)
		status := 0
		if verify {
			_, err := d.GetData(key, val, true)
			if err == nil {
				status = StatusOk
			} else if os.IsNotExist(err) {
				status = StatusMissing
			} else if _, ok := err.(InvalidBlock); ok || err == io.EOF || err == io.ErrUnexpectedEOF {
				status = StatusInvalid
			} else {
				status = StatusError
			}
		}
		out <- &ListRes{key.Bytes()[1:], val.StripData(), status}
	}
	return nil
}

func List(d *Datastore, out chan<- *ListRes) error { return list(d, out, false) }

func Verify(d *Datastore, out chan<- *ListRes) error { return list(d, out, true) }
