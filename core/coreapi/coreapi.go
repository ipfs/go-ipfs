package coreapi

import (
	"context"
	"strings"

	core "github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	ipfspath "github.com/ipfs/go-ipfs/path"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
)

type CoreAPI struct {
	node *core.IpfsNode
}

func NewCoreAPI(n *core.IpfsNode) coreiface.CoreAPI {
	api := &CoreAPI{n}
	return api
}

func (api *CoreAPI) Unixfs() coreiface.UnixfsAPI {
	return (*UnixfsAPI)(api)
}

// TODO: also return path here
func (api *CoreAPI) ResolveNode(ctx context.Context, p coreiface.Path) (coreiface.Path, coreiface.Node, error) {
	p, err := api.ResolvePath(ctx, p)
	if err != nil {
		return nil, nil, err
	}

	node, err := api.node.DAG.Get(ctx, p.Cid())
	if err != nil {
		return nil, nil, err
	}
	return p, node, nil
}

// TODO: store all of ipfspath.Resolver.ResolvePathComponents() in Path
func (api *CoreAPI) ResolvePath(ctx context.Context, p coreiface.Path) (coreiface.Path, error) {
	if p.Resolved() {
		return p, nil
	}

	r := &ipfspath.Resolver{
		DAG:         api.node.DAG,
		ResolveOnce: uio.ResolveUnixfsOnce,
	}

	p2 := ipfspath.FromString(p.String())
	node, err := core.Resolve(ctx, api.node.Namesys, r, p2)
	if err == core.ErrNoNamesys {
		return nil, coreiface.ErrOffline
	} else if _, ok := err.(ipfspath.ErrNoLink); ok {
		return nil, coreiface.ErrNotFound
	} else if err != nil {
		return nil, err
	}

	var root *cid.Cid
	if p2.IsJustAKey() {
		root = node.Cid()
	}

	return ResolvedPath(p.String(), node.Cid(), root), nil
}

// Implements coreiface.Path
type path struct {
	path ipfspath.Path
	cid  *cid.Cid
	root *cid.Cid
}

func ParsePath(p string) (coreiface.Path, error) {
	pp, err := ipfspath.ParsePath(p)
	if err != nil {
		return nil, err
	}
	var root *cid.Cid
	r, _, err := ipfspath.SplitAbsPath(pp)
	if err == nil {
		root = r
	}
	return &path{path: pp, root: root}, nil
}

func ParseCid(c *cid.Cid) coreiface.Path {
	return &path{path: ipfspath.FromCid(c), cid: c, root: c}
}

func ResolvedPath(p string, c *cid.Cid, r *cid.Cid) coreiface.Path {
	return &path{path: ipfspath.FromString(p), cid: c, root: r}
}

func (p *path) String() string       { return p.path.String() }
func (p *path) Components() []string { return p.path.Segments() }
func (p *path) Cid() *cid.Cid        { return p.cid }
func (p *path) RootCid() *cid.Cid    { return p.root }
func (p *path) Resolved() bool       { return p.cid != nil }

func JoinComponents(comps []string) string {
	return strings.Join(comps, "/")
}

func SplitPath(p string) []string {
	return strings.Split(p, "/")
}
