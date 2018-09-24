package dagutils

import (
	"context"
	"errors"

	dag "gx/ipfs/QmVB15p7qNVQQGC5RQcAQAovDFaQpRNqorbRwpvdNjHmVC/go-merkledag"
	bserv "gx/ipfs/QmWAU1Etv448cx9GLohtr27vnVC87amqE7fN4Hf4hcLDQY/go-blockservice"
	path "gx/ipfs/Qmc17MNY1xUgiE2nopbi6KATWau9qcGZtdmKKuXvFMVUgc/go-path"

	ds "gx/ipfs/QmSpg1CvpXQQow5ernt1gNBXaXV6yxyNqi7XoeerWfzB5w/go-datastore"
	syncds "gx/ipfs/QmSpg1CvpXQQow5ernt1gNBXaXV6yxyNqi7XoeerWfzB5w/go-datastore/sync"
	offline "gx/ipfs/QmcRC35JF2pJQneAxa5LdQBQRumWggccWErogSrCkS1h8T/go-ipfs-exchange-offline"
	ipld "gx/ipfs/QmdDXJs4axxefSPgK6Y1QhpJWKuDPnGJiqgq4uncb4rFHL/go-ipld-format"
	bstore "gx/ipfs/QmegPGspn3RpTMQ23Fd3GVVMopo1zsEMurudbFMZ5UXBLH/go-ipfs-blockstore"
)

// Editor represents a ProtoNode tree editor and provides methods to
// modify it.
type Editor struct {
	root *dag.ProtoNode

	// tmp is a temporary in memory (for now) dagstore for all of the
	// intermediary nodes to be stored in
	tmp ipld.DAGService

	// src is the dagstore with *all* of the data on it, it is used to pull
	// nodes from for modification (nil is a valid value)
	src ipld.DAGService
}

// NewMemoryDagService returns a new, thread-safe in-memory DAGService.
func NewMemoryDagService() ipld.DAGService {
	// build mem-datastore for editor's intermediary nodes
	bs := bstore.NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	bsrv := bserv.New(bs, offline.Exchange(bs))
	return dag.NewDAGService(bsrv)
}

// NewDagEditor returns an ProtoNode editor.
//
// * root is the node to be modified
// * source is the dagstore to pull nodes from (optional)
func NewDagEditor(root *dag.ProtoNode, source ipld.DAGService) *Editor {
	return &Editor{
		root: root,
		tmp:  NewMemoryDagService(),
		src:  source,
	}
}

// GetNode returns the a copy of the root node being edited.
func (e *Editor) GetNode() *dag.ProtoNode {
	return e.root.Copy().(*dag.ProtoNode)
}

// GetDagService returns the DAGService used by this editor.
func (e *Editor) GetDagService() ipld.DAGService {
	return e.tmp
}

func addLink(ctx context.Context, ds ipld.DAGService, root *dag.ProtoNode, childname string, childnd ipld.Node) (*dag.ProtoNode, error) {
	if childname == "" {
		return nil, errors.New("cannot create link with no name")
	}

	// ensure that the node we are adding is in the dagservice
	err := ds.Add(ctx, childnd)
	if err != nil {
		return nil, err
	}

	//_ = ds.Remove(ctx, root.Cid())

	// ensure no link with that name already exists
	_ = root.RemoveNodeLink(childname) // ignore error, only option is ErrNotFound

	if err := root.AddNodeLink(childname, childnd); err != nil {
		return nil, err
	}

	if err := ds.Add(ctx, root); err != nil {
		return nil, err
	}
	return root, nil
}

// InsertNodeAtPath inserts a new node in the tree and replaces the current root with the new one.
func (e *Editor) InsertNodeAtPath(ctx context.Context, pth string, toinsert ipld.Node, create func() *dag.ProtoNode) error {
	splpath := path.SplitList(pth)
	nd, err := e.insertNodeAtPath(ctx, e.root, splpath, toinsert, create)
	if err != nil {
		return err
	}
	e.root = nd
	return nil
}

func (e *Editor) insertNodeAtPath(ctx context.Context, root *dag.ProtoNode, path []string, toinsert ipld.Node, create func() *dag.ProtoNode) (*dag.ProtoNode, error) {
	if len(path) == 1 {
		return addLink(ctx, e.tmp, root, path[0], toinsert)
	}

	nd, err := root.GetLinkedProtoNode(ctx, e.tmp, path[0])
	if err != nil {
		// if 'create' is true, we create directories on the way down as needed
		if err == dag.ErrLinkNotFound && create != nil {
			nd = create()
			err = nil // no longer an error case
		} else if err == ipld.ErrNotFound {
			// try finding it in our source dagstore
			nd, err = root.GetLinkedProtoNode(ctx, e.src, path[0])
		}

		// if we receive an ErrNotFound, then our second 'GetLinkedNode' call
		// also fails, we want to error out
		if err != nil {
			return nil, err
		}
	}

	ndprime, err := e.insertNodeAtPath(ctx, nd, path[1:], toinsert, create)
	if err != nil {
		return nil, err
	}

	//_ = e.tmp.Remove(ctx, root.Cid())

	_ = root.RemoveNodeLink(path[0])
	err = root.AddNodeLink(path[0], ndprime)
	if err != nil {
		return nil, err
	}

	err = e.tmp.Add(ctx, root)
	if err != nil {
		return nil, err
	}

	return root, nil
}

// RmLink removes the link with the given name and updates the root node of
// the editor.
func (e *Editor) RmLink(ctx context.Context, pth string) error {
	splpath := path.SplitList(pth)
	nd, err := e.rmLink(ctx, e.root, splpath)
	if err != nil {
		return err
	}
	e.root = nd
	return nil
}

func (e *Editor) rmLink(ctx context.Context, root *dag.ProtoNode, path []string) (*dag.ProtoNode, error) {
	if len(path) == 1 {
		// base case, remove node in question
		err := root.RemoveNodeLink(path[0])
		if err != nil {
			return nil, err
		}

		err = e.tmp.Add(ctx, root)
		if err != nil {
			return nil, err
		}

		return root, nil
	}

	// search for node in both tmp dagstore and source dagstore
	nd, err := root.GetLinkedProtoNode(ctx, e.tmp, path[0])
	if err == ipld.ErrNotFound {
		nd, err = root.GetLinkedProtoNode(ctx, e.src, path[0])
	}

	if err != nil {
		return nil, err
	}

	nnode, err := e.rmLink(ctx, nd, path[1:])
	if err != nil {
		return nil, err
	}

	//e.tmp.Remove(ctx, root.Cid())

	_ = root.RemoveNodeLink(path[0])
	err = root.AddNodeLink(path[0], nnode)
	if err != nil {
		return nil, err
	}

	err = e.tmp.Add(ctx, root)
	if err != nil {
		return nil, err
	}

	return root, nil
}

// Finalize writes the new DAG to the given DAGService and returns the modified
// root node.
func (e *Editor) Finalize(ctx context.Context, ds ipld.DAGService) (*dag.ProtoNode, error) {
	nd := e.GetNode()
	err := copyDag(ctx, nd, e.tmp, ds)
	return nd, err
}

func copyDag(ctx context.Context, nd ipld.Node, from, to ipld.DAGService) error {
	// TODO(#4609): make this batch.
	err := to.Add(ctx, nd)
	if err != nil {
		return err
	}

	for _, lnk := range nd.Links() {
		child, err := lnk.GetNode(ctx, from)
		if err != nil {
			if err == ipld.ErrNotFound {
				// not found means we didnt modify it, and it should
				// already be in the target datastore
				continue
			}
			return err
		}

		err = copyDag(ctx, child, from, to)
		if err != nil {
			return err
		}
	}
	return nil
}
