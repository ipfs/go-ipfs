package corehttp

import (
	"net/http"
	"os"

	commands "github.com/ipfs/go-ipfs/commands"
	cmdsHttp "github.com/ipfs/go-ipfs/commands/http"
	core "github.com/ipfs/go-ipfs/core"
	corecommands "github.com/ipfs/go-ipfs/core/commands"
)

const (
	// TODO rename
	originEnvKey = "API_ORIGIN"
)

func CommandsOption(cctx commands.Context) ServeOption {
	return func(n *core.IpfsNode, mux *http.ServeMux) (*http.ServeMux, error) {
		origin := os.Getenv(originEnvKey)
		cmdHandler := cmdsHttp.NewHandler(cctx, corecommands.Root, origin)
		mux.Handle(cmdsHttp.ApiPath+"/", cmdHandler)
		return mux, nil
	}
}
