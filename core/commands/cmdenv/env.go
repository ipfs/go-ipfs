package cmdenv

import (
	"fmt"
	"strings"

	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"

	cmds "github.com/ipfs/go-ipfs-cmds"
	config "github.com/ipfs/go-ipfs-config"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	options "github.com/ipfs/interface-go-ipfs-core/options"
)

var log = logging.Logger("core/commands/cmdenv")

// GetNode extracts the node from the environment.
func GetNode(env interface{}) (*core.IpfsNode, error) {
	ctx, ok := env.(*commands.Context)
	if !ok {
		return nil, fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	return ctx.GetNode()
}

// GetApi extracts CoreAPI instance from the environment.
func GetApi(env cmds.Environment, req *cmds.Request) (coreiface.CoreAPI, error) {
	ctx, ok := env.(*commands.Context)
	if !ok {
		return nil, fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	offline, _ := req.Options["offline"].(bool)
	if !offline {
		offline, _ = req.Options["local"].(bool)
		if offline {
			log.Errorf("Command '%s', --local is deprecated, use --offline instead", strings.Join(req.Path, " "))
		}
	}
	api, err := ctx.GetAPI()
	if err != nil {
		return nil, err
	}
	if offline {
		return api.WithOptions(options.Api.Offline(offline))
	}

	return api, nil
}

// GetConfig extracts the config from the environment.
func GetConfig(env cmds.Environment) (*config.Config, error) {
	ctx, ok := env.(*commands.Context)
	if !ok {
		return nil, fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	return ctx.GetConfig()
}

// GetConfigRoot extracts the config root from the environment
func GetConfigRoot(env cmds.Environment) (string, error) {
	ctx, ok := env.(*commands.Context)
	if !ok {
		return "", fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	return ctx.RepoPath, nil
}
