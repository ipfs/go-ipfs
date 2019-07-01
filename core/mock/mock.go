package coremock

import (
	"context"

	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
)

// NewMockNode constructs an IpfsNode for use in tests.
func NewMockNode() (*core.IpfsNode, error) {
	ctx := context.Background()

	// effectively offline, only peer in its network
	api, err := coreapi.New(coreapi.Ctx(ctx),
		coreapi.Online(),
		coreapi.MockHost(mocknet.New(ctx)))

	if err != nil {
		return nil, err
	}

	// nolint
	return api.Node(), nil
}
