// Package path implements utilities for resolving paths within ipfs.
package path

import (
	"context"
	"errors"
	"fmt"
	"time"

	dag "github.com/ipfs/go-ipfs/merkledag"

	"gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	node "gx/ipfs/QmPN7cwmpcc4DWXb4KTB9dNAJgjuPY69h3npsMfhRrQL9c/go-ipld-format"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
)

var log = logging.Logger("path")

// ErrNoComponents is returned if a path contains no components.
// Paths after a protocol must contain at least one component.
var ErrNoComponents = errors.New(
	"path must contain at least one component")

// ErrNoLink is returned when a link is not found in a path.
type ErrNoLink struct {
	Name string
	Node *cid.Cid
}

func (e ErrNoLink) Error() string {
	return fmt.Sprintf("no link named %q under %s", e.Name, e.Node.String())
}

// Resolver provides path resolution for IPFS.
// It has a pointer to a DAGService, which is used to resolve nodes.
// TODO: now that this is more modular, try to unify this code with the resolvers in namesys.
type Resolver struct {
	DAG dag.DAGService

	ResolveOnce func(ctx context.Context, ds dag.DAGService, nd node.Node, names []string) (*node.Link, []string, error)
}

// NewBasicResolver creates a new resolve for the given DAGService, using
// ResolveSingle as its resolution funciton.
func NewBasicResolver(ds dag.DAGService) *Resolver {
	return &Resolver{
		DAG:         ds,
		ResolveOnce: ResolveSingle,
	}
}

// SplitAbsPath cleans up and splits fpath. It extracts the first component
// (which must be a Multihash) and returns the remaining path separately.
func SplitAbsPath(fpath Path) (*cid.Cid, []string, error) {

	log.Debugf("Resolve: '%s'", fpath)

	parts := fpath.Segments()
	if parts[0] == "ipfs" {
		parts = parts[1:]
	}

	// if nothing, bail.
	if len(parts) == 0 {
		return nil, nil, ErrNoComponents
	}

	c, err := cid.Decode(parts[0])
	// first element in the path is a cid
	if err != nil {
		log.Debug("given path element is not a cid.\n")
		return nil, nil, err
	}

	return c, parts[1:], nil
}

// ResolvePath fetches the node for given path. It returns the node for
// the last segment of the given path, i.e. the last item returned by
// ResolvePathComponents.
func (r *Resolver) ResolvePath(ctx context.Context, fpath Path) (node.Node, error) {
	// validate path
	if err := fpath.IsValid(); err != nil {
		return nil, err
	}

	nodes, err := r.ResolvePathComponents(ctx, fpath)
	if err != nil || nodes == nil {
		return nil, err
	}
	return nodes[len(nodes)-1], nil
}

// ResolveSingle simply resolves one hop of a path through a graph with no
// extra context (does not opaquely resolve through sharded nodes)
func ResolveSingle(ctx context.Context, ds dag.DAGService, nd node.Node, names []string) (*node.Link, []string, error) {
	return nd.ResolveLink(names)
}

// ResolvePathComponents fetches the nodes for each segment of the given path.
// It uses the first path component as a hash (key) of the first node, then
// resolves all other components walking the links, with ResolveLinks.
func (r *Resolver) ResolvePathComponents(ctx context.Context, fpath Path) ([]node.Node, error) {
	evt := log.EventBegin(ctx, "resolvePathComponents", logging.LoggableMap{"fpath": fpath})
	defer evt.Done()

	hash, parts, err := SplitAbsPath(fpath)
	if err != nil {
		evt.Append(logging.LoggableMap{"error": err.Error()})
		return nil, err
	}

	log.Debug("resolve dag get")
	nd, err := r.DAG.Get(ctx, hash)
	if err != nil {
		evt.Append(logging.LoggableMap{"error": err.Error()})
		return nil, err
	}

	return r.ResolveLinks(ctx, nd, parts)
}

// ResolveToLastNode fetches the last node for the segments of the given path.
// The first path component is treated as the hash (key) of the first node.  The
// function then recursively resolves the node for each segment until the last
// node nd is found.
//
// The segments of the path after the segment corresponding to nd are returned
// as a slice of strings in the parameter rest.
//
// ResolveToLastNode([]string{"foo", "bar", "baz"})
// would retrieve "baz" in ("bar" in ("foo" in nd.Links).Links).Links,
// and return a nil rest variable.
func (r *Resolver) ResolveToLastNode(ctx context.Context, fpath Path) (
	nd node.Node, rest []string, err error,
) {
	hash, parts, err := SplitAbsPath(fpath)
	if err != nil {
		return nil, nil, err
	}

	log.Debug("resolve dag get")
	nd, err = r.DAG.Get(ctx, hash)
	if err != nil {
		return nil, nil, err
	}

	for len(parts) > 0 {
		// Try to resolve the path through this node to a link.  If a link
		// is found, then we have not yet reached the last node.
		val, rest, err := nd.Resolve(parts)
		if err != nil {
			return nil, nil, err
		}

		// The last node has been found, so exit.
		link, ok := val.(*node.Link)
		if !ok {
			return nd, rest, nil
		}

		// The last node has not been found, so try the next node.
		parts = rest
		nd, err = link.GetNode(ctx, r.DAG)
		if err != nil {
			return nil, nil, err
		}
	}

	// Path was followed through to the end, so exit.
	return nd, nil, nil
}

// ResolveLinks iteratively resolves names by walking the link hierarchy.
// Every node is fetched from the DAGService, resolving the next name.
// It returns the list of nodes forming the path, starting with ndd. This list
// is never empty.
//
// ResolveLinks(ndd, []string{"foo", "bar", "baz"})
// would retrieve "baz" in ("bar" in ("foo" in nd.Links).Links).Links
func (r *Resolver) ResolveLinks(ctx context.Context, ndd node.Node, names []string) ([]node.Node, error) {

	evt := log.EventBegin(ctx, "resolveLinks", logging.LoggableMap{"names": names})
	defer evt.Done()
	result := make([]node.Node, 0, len(names)+1)
	result = append(result, ndd)
	nd := ndd // duplicate argument workaround

	// Iterate through each of the path's components.
	for len(names) > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Minute)
		defer cancel()

		lnk, rest, err := r.ResolveOnce(ctx, r.DAG, nd, names)
		if err == dag.ErrLinkNotFound {
			evt.Append(logging.LoggableMap{"error": err.Error()})
			return result, ErrNoLink{Name: names[0], Node: nd.Cid()}
		} else if err != nil {
			evt.Append(logging.LoggableMap{"error": err.Error()})
			return result, err
		}

		nextnode, err := lnk.GetNode(ctx, r.DAG)
		if err != nil {
			evt.Append(logging.LoggableMap{"error": err.Error()})
			return result, err
		}

		nd = nextnode
		result = append(result, nextnode)
		names = rest
	}
	return result, nil
}
