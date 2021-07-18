package hub

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

type Client struct {
	topic *pubsub.Topic
}

func NewClient(topic *pubsub.Topic) (*Client, error) {
	res := &Client{
		topic: topic,
	}

	return res, nil
}

func (c *Client) Provide(ctx context.Context, cid cid.Cid, b bool) error {
	return nil
}

func (c *Client) FindProvidersAsync(ctx context.Context, cid cid.Cid, i int) <-chan peer.AddrInfo {
	_ = c.topic.Publish(ctx, cid.Bytes())
	ch := make(chan peer.AddrInfo)
	close(ch)
	return ch
}

func (c *Client) Close() error {
	return c.topic.Close()
}

var _ routing.ContentRouting = (*Client)(nil)
var _ io.Closer = (*Client)(nil)
