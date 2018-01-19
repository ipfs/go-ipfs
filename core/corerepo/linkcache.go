package corerepo

import (
	context "context"
	core "github.com/ipfs/go-ipfs/core"
	ds "gx/ipfs/QmdHG8MAuARdGHxx4rPQASLcvhz24fzjSQq7AJRAQEorq5/go-datastore"
	dsq "gx/ipfs/QmdHG8MAuARdGHxx4rPQASLcvhz24fzjSQq7AJRAQEorq5/go-datastore/query"
)

// FlushLinkCache flushes link cache, deleting all keys in it
func FlushLinkCache(ctx context.Context, n *core.IpfsNode) error {
	d := n.Repo.Datastore()
	q := dsq.Query{KeysOnly: true, Prefix: "/local/links/"}
	qr, err := d.Query(q)
	if err != nil {
		return err
	}
	for result := range qr.Next() {
		if result.Error != nil {
			return result.Error
		}
		d.Delete(ds.NewKey(result.Entry.Key))
	}
	return nil
}
