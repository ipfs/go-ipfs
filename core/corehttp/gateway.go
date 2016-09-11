package corehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"

	core "github.com/ipfs/go-ipfs/core"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	config "github.com/ipfs/go-ipfs/repo/config"
	id "gx/ipfs/QmXnaDLonE9YBTVDdWBM6Jb5YxxmW1MHMkXzgsnu1jTEmK/go-libp2p/p2p/protocol/identify"
)

type GatewayConfig struct {
	Headers      map[string][]string
	Writable     bool
	PathPrefixes []string
}

type apiOption func(context.Context) coreiface.UnixfsAPI

func GatewayOption(paths ...string) ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		cfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}

		apiOpt := func(ctx context.Context) coreiface.UnixfsAPI {
			api := &coreapi.UnixfsAPI{Context: ctx, Node: n}
			return api
		}
		gateway := newGatewayHandler(n, GatewayConfig{
			Headers:      cfg.Gateway.HTTPHeaders,
			Writable:     cfg.Gateway.Writable,
			PathPrefixes: cfg.Gateway.PathPrefixes,
		}, apiOpt)

		for _, p := range paths {
			mux.Handle(p+"/", gateway)
		}
		return mux, nil
	}
}

func VersionOption() ServeOption {
	return func(_ *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Commit: %s\n", config.CurrentCommit)
			fmt.Fprintf(w, "Client Version: %s\n", id.ClientVersion)
			fmt.Fprintf(w, "Protocol Version: %s\n", id.LibP2PVersion)
		})
		return mux, nil
	}
}
