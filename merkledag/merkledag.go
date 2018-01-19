// package merkledag implements the IPFS Merkle DAG datastructures.
package merkledag

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	bserv "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"

	pb "github.com/ipfs/go-ipfs/merkledag/pb"
	node "gx/ipfs/QmNwUEK7QbwSqyKBu3mMtToo8SUc6wQJ7gdZq4gGGJqfnf/go-ipld-format"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
	ipldcbor "gx/ipfs/QmeZv9VXw2SfVbX55LV6kGTWASKBc9ZxAVqGBeJcDGdoXy/go-ipld-cbor"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	ds "gx/ipfs/QmdHG8MAuARdGHxx4rPQASLcvhz24fzjSQq7AJRAQEorq5/go-datastore"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
)

var log = logging.Logger("merkledag")

// TODO: We should move these registrations elsewhere. Really, most of the IPLD
// functionality should go in a `go-ipld` repo but that will take a lot of work
// and design.
func init() {
	node.Register(cid.DagProtobuf, DecodeProtobufBlock)
	node.Register(cid.Raw, DecodeRawBlock)
	node.Register(cid.DagCBOR, ipldcbor.DecodeBlock)
}

var ErrNotFound = fmt.Errorf("merkledag: not found")

// DAGService is an IPFS Merkle DAG service.
type DAGService interface {
	Add(node.Node) (*cid.Cid, error)
	Get(context.Context, *cid.Cid) (node.Node, error)
	Fetch(context.Context, *cid.Cid) error
	Remove(node.Node) error

	// GetMany returns a channel of NodeOption given
	// a set of CIDs.
	GetMany(context.Context, []*cid.Cid) <-chan *NodeOption

	Batch() *Batch

	LinkService
}

type LinkService interface {
	// GetLinks return all links for a node.  The complete node does not
	// necessarily have to exist locally, or at all.  For example, raw
	// leaves cannot possibly have links so there is no need to look
	// at the node.
	GetLinks(context.Context, *cid.Cid) ([]*node.Link, error)

	GetOfflineLinkService() LinkService
}

// NewDAGService is shortcut for creating new dagService
func NewDAGService(bs bserv.BlockService, d ds.Datastore) *dagService {
	return &dagService{Blocks: bs, Data: d}
}

// dagService is an IPFS Merkle DAG service.
// - the root is virtual (like a forest)
// - stores nodes' data in a BlockService
// TODO: should cache Nodes that are in memory, and be
//       able to free some of them when vm pressure is high
type dagService struct {
	Blocks bserv.BlockService
	Data   ds.Datastore
}

// Add adds a node to the dagService, storing the block in the BlockService
func (n *dagService) Add(nd node.Node) (*cid.Cid, error) {
	if n == nil { // FIXME remove this assertion. protect with constructor invariant
		return nil, fmt.Errorf("dagService is nil")
	}

	return n.Blocks.AddBlock(nd)
}

func (n *dagService) Batch() *Batch {
	return &Batch{
		ds:            n,
		commitResults: make(chan error, ParallelBatchCommits),
		MaxSize:       8 << 20,

		// By default, only batch up to 128 nodes at a time.
		// The current implementation of flatfs opens this many file
		// descriptors at the same time for the optimized batch write.
		MaxBlocks: 128,
	}
}

// Get retrieves a node from the dagService, fetching the block in the BlockService
func (n *dagService) Get(ctx context.Context, c *cid.Cid) (node.Node, error) {
	if n == nil {
		return nil, fmt.Errorf("dagService is nil")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	b, err := n.Blocks.GetBlock(ctx, c)
	if err != nil {
		if err == bserv.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("Failed to get block for %s: %v", c, err)
	}

	return node.Decode(b)
}

// Fetch ensures node is available in local storage
func (n *dagService) Fetch(ctx context.Context, c *cid.Cid) error {
	if n == nil {
		return fmt.Errorf("dagService is nil")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := n.Blocks.FetchBlock(ctx, c)

	if err != nil {
		if err == bserv.ErrNotFound {
			return ErrNotFound
		}
		return fmt.Errorf("Failed to get block for %s: %v", c, err)
	}
	return nil
}

func marshalCachedLinks(ls []*node.Link) []byte {
	var buffer bytes.Buffer

	pbw := ggio.NewDelimitedWriter(&buffer)
	for _, l := range ls {
		pbl := intermediateMarshal(l)
		pbw.WriteMsg(pbl)
	}
	return buffer.Bytes()
}

func unmarshalCachedLinks(data []byte) ([]*node.Link, error) {
	var links []*node.Link

	buffer := bytes.NewBuffer(data)
	pbr := ggio.NewDelimitedReader(buffer, 1024)
	var pbl pb.PBLink
	var err error
	for err = pbr.ReadMsg(&pbl); err == nil; err = pbr.ReadMsg(&pbl) {
		link, errU := intermediateUnmarshal(&pbl)
		if errU != nil {
			return nil, errU
		}
		links = append(links, link)
	}
	if err != io.EOF {
		return nil, err
	}
	return links, nil

}

// GetLinks return the links for the node, the node doesn't necessarily have
// to exist locally.
func (n *dagService) GetLinks(ctx context.Context, c *cid.Cid) ([]*node.Link, error) {
	if c.Type() == cid.Raw {
		return nil, nil
	}

	dKey := ds.NewKey(fmt.Sprint("/local/links/", c.String()))
	linksI, err := n.Data.Get(dKey)
	if err == ds.ErrNotFound {
		node, err := n.Get(ctx, c)
		if err != nil {
			return nil, err
		}
		n.Data.Put(dKey, marshalCachedLinks(node.Links()))
		return node.Links(), nil
	} else if err == nil {

		linksRaw, ok := linksI.([]byte)
		if ok != true {
			return nil, fmt.Errorf("loading link cache failed: %s was not bytes", dKey)
		}

		links, err := unmarshalCachedLinks(linksRaw)
		if err != nil {
			return nil, err
		}
		return links, nil
	} else {
		return nil, err
	}
}

func (n *dagService) GetOfflineLinkService() LinkService {
	if n.Blocks.Exchange().IsOnline() {
		bsrv := bserv.New(n.Blocks.Blockstore(), offline.Exchange(n.Blocks.Blockstore()))
		return NewDAGService(bsrv, n.Data)
	} else {
		return n
	}
}

func (n *dagService) Remove(nd node.Node) error {
	return n.Blocks.DeleteBlock(nd)
}

// GetLinksDirect creates a function to get the links for a node, from
// the node, bypassing the LinkService.  If the node does not exist
// locally (and can not be retrieved) an error will be returned.
func GetLinksDirect(serv node.NodeGetter) GetLinks {
	return func(ctx context.Context, c *cid.Cid) ([]*node.Link, error) {
		node, err := serv.Get(ctx, c)
		if err != nil {
			if err == bserv.ErrNotFound {
				err = ErrNotFound
			}
			return nil, err
		}
		return node.Links(), nil
	}
}

type sesGetter struct {
	bs *bserv.Session
}

func (sg *sesGetter) Get(ctx context.Context, c *cid.Cid) (node.Node, error) {
	blk, err := sg.bs.GetBlock(ctx, c)
	switch err {
	case bserv.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	case nil:
		// noop
	}

	return node.Decode(blk)
}

// FetchGraph fetches all nodes that are children of the given node
func FetchGraph(ctx context.Context, root *cid.Cid, serv DAGService) error {
	fetchingVisitor := FetchingVisitor(ctx, cid.NewSet(), serv)
	var result error

	var ng node.NodeGetter = serv
	ds, ok := serv.(*dagService)
	if ok {
		ng = &sesGetter{bserv.NewSession(ctx, ds.Blocks)}
	}

	v, _ := ctx.Value("progress").(*ProgressTracker)

	if v == nil {
		result = EnumerateChildrenAsync(ctx, serv.GetLinks, root, fetchingVisitor)
	} else {
		visit := func(c *cid.Cid) (bool, error) {
			visitResult, err := fetchingVisitor(c)
			if err != nil {
				return false, err
			}
			if visitResult {
				v.Increment()
				return true, nil
			} else {
				return false, nil
			}

		}
		result = EnumerateChildrenAsync(ctx, GetLinksDirect(ng), root, visit)
	}

	return result
}

// FindLinks searches this nodes links for the given key,
// returns the indexes of any links pointing to it
func FindLinks(links []*cid.Cid, c *cid.Cid, start int) []int {
	var out []int
	for i, lnk_c := range links[start:] {
		if c.Equals(lnk_c) {
			out = append(out, i+start)
		}
	}
	return out
}

type NodeOption struct {
	Node node.Node
	Err  error
}

func (ds *dagService) GetMany(ctx context.Context, keys []*cid.Cid) <-chan *NodeOption {
	out := make(chan *NodeOption, len(keys))
	blocks := ds.Blocks.GetBlocks(ctx, keys)
	var count int

	go func() {
		defer close(out)
		for {
			select {
			case b, ok := <-blocks:
				if !ok {
					if count != len(keys) {
						out <- &NodeOption{Err: fmt.Errorf("failed to fetch all nodes")}
					}
					return
				}

				nd, err := node.Decode(b)
				if err != nil {
					out <- &NodeOption{Err: err}
					return
				}

				out <- &NodeOption{Node: nd}
				count++

			case <-ctx.Done():
				out <- &NodeOption{Err: ctx.Err()}
				return
			}
		}
	}()
	return out
}

// GetDAG will fill out all of the links of the given Node.
// It returns a channel of nodes, which the caller can receive
// all the child nodes of 'root' on, in proper order.
func GetDAG(ctx context.Context, ds DAGService, root node.Node) []NodeGetter {
	var cids []*cid.Cid
	for _, lnk := range root.Links() {
		cids = append(cids, lnk.Cid)
	}

	return GetNodes(ctx, ds, cids)
}

// GetNodes returns an array of 'NodeGetter' promises, with each corresponding
// to the key with the same index as the passed in keys
func GetNodes(ctx context.Context, ds DAGService, keys []*cid.Cid) []NodeGetter {

	// Early out if no work to do
	if len(keys) == 0 {
		return nil
	}

	promises := make([]NodeGetter, len(keys))
	for i := range keys {
		promises[i] = newNodePromise(ctx)
	}

	dedupedKeys := dedupeKeys(keys)
	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		nodechan := ds.GetMany(ctx, dedupedKeys)

		for count := 0; count < len(keys); {
			select {
			case opt, ok := <-nodechan:
				if !ok {
					for _, p := range promises {
						p.Fail(ErrNotFound)
					}
					return
				}

				if opt.Err != nil {
					for _, p := range promises {
						p.Fail(opt.Err)
					}
					return
				}

				nd := opt.Node
				is := FindLinks(keys, nd.Cid(), 0)
				for _, i := range is {
					count++
					promises[i].Send(nd)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return promises
}

// Remove duplicates from a list of keys
func dedupeKeys(cids []*cid.Cid) []*cid.Cid {
	out := make([]*cid.Cid, 0, len(cids))
	set := cid.NewSet()
	for _, c := range cids {
		if set.Visit(c) {
			out = append(out, c)
		}
	}
	return out
}

func newNodePromise(ctx context.Context) NodeGetter {
	return &nodePromise{
		recv: make(chan node.Node, 1),
		ctx:  ctx,
		err:  make(chan error, 1),
	}
}

type nodePromise struct {
	cache node.Node
	clk   sync.Mutex
	recv  chan node.Node
	ctx   context.Context
	err   chan error
}

// NodeGetter provides a promise like interface for a dag Node
// the first call to Get will block until the Node is received
// from its internal channels, subsequent calls will return the
// cached node.
type NodeGetter interface {
	Get(context.Context) (node.Node, error)
	Fail(err error)
	Send(node.Node)
}

func (np *nodePromise) Fail(err error) {
	np.clk.Lock()
	v := np.cache
	np.clk.Unlock()

	// if promise has a value, don't fail it
	if v != nil {
		return
	}

	np.err <- err
}

func (np *nodePromise) Send(nd node.Node) {
	var already bool
	np.clk.Lock()
	if np.cache != nil {
		already = true
	}
	np.cache = nd
	np.clk.Unlock()

	if already {
		panic("sending twice to the same promise is an error!")
	}

	np.recv <- nd
}

func (np *nodePromise) Get(ctx context.Context) (node.Node, error) {
	np.clk.Lock()
	c := np.cache
	np.clk.Unlock()
	if c != nil {
		return c, nil
	}

	select {
	case nd := <-np.recv:
		return nd, nil
	case <-np.ctx.Done():
		return nil, np.ctx.Err()
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-np.err:
		return nil, err
	}
}

type GetLinks func(context.Context, *cid.Cid) ([]*node.Link, error)

type CidVisitor func(*cid.Cid) (bool, error)

// OnceVisitor is helper function used with EnumerateChildren/EnumerateChildrenAsync to visit all nodes once and check if they are present in storage
func OnceVisitor(visitation *cid.Set) CidVisitor {
	var setlk sync.Mutex

	return func(vcid *cid.Cid) (bool, error) {
		setlk.Lock()
		defer setlk.Unlock()
		return visitation.Visit(vcid), nil
	}
}

// FetchingVisitor is helper function used with EnumerateChildren/EnumerateChildrenAsync to fetch all enumerated nodes
func FetchingVisitor(ctx context.Context, visitation *cid.Set, serv DAGService) CidVisitor {

	var setlk sync.Mutex

	return func(vcid *cid.Cid) (bool, error) {
		err := serv.Fetch(ctx, vcid)

		setlk.Lock()
		defer setlk.Unlock()
		if err == nil {
			return visitation.Visit(vcid), nil
		} else {
			return false, err
		}
	}
}

// EnumerateChildren will walk the dag below the given root node and add all
// unseen children to the passed in set.
// TODO: parallelize to avoid disk latency perf hits?
func EnumerateChildren(ctx context.Context, getLinks GetLinks, root *cid.Cid, visit CidVisitor) error {
	links, err := getLinks(ctx, root)
	if err != nil {
		return err
	}
	for _, lnk := range links {
		c := lnk.Cid
		vis, err := visit(c)
		if err != nil {
			return err
		}
		if vis {
			err = EnumerateChildren(ctx, getLinks, c, visit)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type ProgressTracker struct {
	Total int
	lk    sync.Mutex
}

func (p *ProgressTracker) DeriveContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, "progress", p)
}
func (p *ProgressTracker) Increment() {
	p.lk.Lock()
	defer p.lk.Unlock()
	p.Total++
}
func (p *ProgressTracker) Value() int {
	p.lk.Lock()
	defer p.lk.Unlock()
	return p.Total
}

// FetchGraphConcurrency is total number of concurrent fetches that
// 'fetchNodes' will start at a time
var FetchGraphConcurrency = 8

// EnumerateChildrenAsync is asynchronous version of EnumerateChildren
// visit must be thread-safe
func EnumerateChildrenAsync(ctx context.Context, getLinks GetLinks, c *cid.Cid, visit CidVisitor) error {
	feed := make(chan *cid.Cid)
	out := make(chan []*cid.Cid)
	done := make(chan struct{})

	errChan := make(chan error)
	fetchersCtx, cancel := context.WithCancel(ctx)

	defer cancel()

	for i := 0; i < FetchGraphConcurrency; i++ {
		go func() {
			for ic := range feed {
				unseen, err := visit(ic)
				if err != nil {
					errChan <- err
					cancel()
					return
				}

				if unseen {
					links, err := getLinks(fetchersCtx, ic)
					if err != nil {
						errChan <- err
						cancel()
						return
					}
					cids := make([]*cid.Cid, len(links))
					for i, l := range links {
						cids[i] = l.Cid
					}
					select {
					case out <- cids:
					case <-fetchersCtx.Done():
						return
					}
				}
				select {
				case done <- struct{}{}:
				case <-fetchersCtx.Done():
				}
			}
		}()
	}
	defer close(feed)

	send := feed
	var todobuffer []*cid.Cid
	var inProgress int

	next := c
	for {
		select {
		case send <- next:
			inProgress++
			if len(todobuffer) > 0 {
				next = todobuffer[0]
				todobuffer = todobuffer[1:]
			} else {
				next = nil
				send = nil
			}
		case <-done:
			inProgress--
			if inProgress == 0 && next == nil {
				return nil
			}
		case cids := <-out:
			if len(cids) > 0 {
				if next == nil {
					next = cids[0]
					todobuffer = append(todobuffer, cids[1:]...)
					send = feed
				} else {
					todobuffer = append(todobuffer, cids...)
				}
			}
		case err := <-errChan:
			return err

		case <-ctx.Done():
			return ctx.Err()
		}
	}

}
