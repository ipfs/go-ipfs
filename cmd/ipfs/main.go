// cmd/ipfs implements the primary CLI binary for ipfs
package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	util "github.com/ipfs/go-ipfs/cmd/ipfs/util"
	oldcmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	corecmds "github.com/ipfs/go-ipfs/core/commands"
	repo "github.com/ipfs/go-ipfs/repo"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"

	"github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cli"
	"github.com/ipfs/go-ipfs-cmds/http"
	logging "github.com/ipfs/go-log"
	loggables "github.com/libp2p/go-libp2p-loggables"
)

// log is the command logger
var log = logging.Logger("cmd/ipfs")

const (
	EnvEnableProfiling = "IPFS_PROF"
	cpuProfile         = "ipfs.cpuprof"
	heapProfile        = "ipfs.memprof"
)

// main roadmap:
// - parse the commandline to get a cmdInvocation
// - if user requests help, print it and exit.
// - run the command invocation
// - output the response
// - if anything fails, print error, maybe with help
func main() {
	os.Exit(mainRet())
}

func mainRet() int {
	rand.Seed(time.Now().UnixNano())
	ctx := logging.ContextWithLoggable(context.Background(), loggables.Uuid("session"))
	var err error

	stopFunc, err := profileIfEnabled()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		return 1
	}
	defer stopFunc() // to be executed as late as possible

	intrh, ctx := util.SetupInterruptHandler(ctx)
	defer intrh.Close()

	// Handle `ipfs version` or `ipfs help`
	if len(os.Args) > 1 {
		// Handle `ipfs --version'
		if os.Args[1] == "--version" {
			os.Args[1] = "version"
		}

		// Handle `ipfs help` and `ipfs help <sub-command>`
		if os.Args[1] == "help" {
			if len(os.Args) > 2 {
				os.Args = append(os.Args[:1], os.Args[2:]...)
				// Handle `ipfs help --help`
				// append `--help`,when the command is not `ipfs help --help`
				if os.Args[1] != "--help" {
					os.Args = append(os.Args, "--help")
				}
			} else {
				os.Args[1] = "--help"
			}
		}
	}

	// output depends on executable name passed in os.Args
	// so we need to make sure it's stable
	os.Args[0] = "ipfs"

	err = cli.Run(ctx, Root, os.Args, os.Stdin, os.Stdout, os.Stderr, buildEnv, makeExecutor)
	if err != nil {
		return 1
	}

	// everything went better than expected :)
	return 0
}

// makeExecutor creates new command executor based on context
//
// It will try to use api endpoint if one exists, and fallback to creating a
// local executor
func makeExecutor(req *cmds.Request, env interface{}) (cmds.Executor, error) {
	details := commandDetails(req.Path)
	client, err := maybeApiClient(details, req, env.(*oldcmds.Context))
	if err != nil {
		return nil, err
	}

	var exctr cmds.Executor
	if client != nil && !req.Command.External {
		exctr = client.(cmds.Executor)
	} else {
		exctr = cmds.NewExecutor(req.Root)
	}

	return exctr, nil
}

// buildEnv provides the environment to makeExecutor and commands.
//
// It's called before makeExecutor, and provides it's `env` argument
func buildEnv(ctx context.Context, req *cmds.Request) (cmds.Environment, error) {
	checkDebug(req)
	repoPath, err := getRepoPath(req)
	if err != nil {
		return nil, err
	}
	log.Debugf("config path is %s", repoPath)

	plugins, err := loadPlugins(repoPath)
	if err != nil {
		return nil, err
	}

	builder := &nodeBuilder{
		ctx:      ctx,
		repoPath: repoPath,

		migrate:     false,
		userMigrate: true, // don't ask for migrations
		offline:     true,

		routing: "default",
	}

	// this sets up the function that will initialize the node
	// this is so that we can construct the node lazily.
	return &oldcmds.Context{
		RepoPath:   repoPath,
		LoadConfig: fsrepo.ConfigAt,
		ReqLog:     &oldcmds.ReqLog{},
		Plugins:    plugins,
		ConstructNode: func() (*core.IpfsNode, error) {
			if req == nil {
				return nil, errors.New("constructing node without a request")
			}

			return builder.buildNode()
		},
	}, nil
}

// commandDetails returns a command's details for the command given by |path|.
func commandDetails(path []string) cmdDetails {
	var details cmdDetails
	// find the last command in path that has a cmdDetailsMap entry
	for i := range path {
		if cmdDetails, found := cmdDetailsMap[strings.Join(path[:i+1], "/")]; found {
			details = cmdDetails
		}
	}
	return details
}

// maybeApiClient determines, from command details, whether a
// command ought to be executed on an ipfs daemon.
//
// It returns a client if the command should be executed on a daemon and nil if
// it should be executed on a client. It returns an error if the command must
// NOT be executed on either.
func maybeApiClient(details cmdDetails, req *cmds.Request, cctx *oldcmds.Context) (http.Client, error) {
	path := req.Path
	// root command.
	if len(path) < 1 {
		return nil, nil
	}

	if details.cannotRunOnClient && details.cannotRunOnDaemon {
		return nil, fmt.Errorf("command disabled: %s", path[0])
	}

	if details.doesNotUseRepo && details.canRunOnClient() {
		return nil, nil
	}

	// at this point need to know whether api is running. we defer
	// to this point so that we don't check unnecessarily

	// did user specify an api to use for this command?
	apiAddrOpt, _ := req.Options[corecmds.ApiOption].(string)

	client, err := getAPIClient(req.Context, cctx.RepoPath, apiAddrOpt)
	if err == repo.ErrApiNotRunning {
		if apiAddrOpt != "" && req.Command != daemonCmd {
			// if user SPECIFIED an api, and this cmd is not daemon
			// we MUST use it. so error out.
			return nil, err
		}

		// ok for api not to be running
	} else if err != nil { // some other api error
		return nil, err
	}

	if client != nil {
		if details.cannotRunOnDaemon {
			// check if daemon locked. legacy error text, for now.
			log.Debugf("Command cannot run on daemon. Checking if daemon is locked")
			if daemonLocked, _ := fsrepo.LockedByOtherProcess(cctx.RepoPath); daemonLocked {
				return nil, cmds.ClientError("ipfs daemon is running. please stop it to run this command")
			}
			return nil, nil
		}

		return client, nil
	}

	if details.cannotRunOnClient {
		return nil, cmds.ClientError("must run on the ipfs daemon")
	}

	return nil, nil
}

func getRepoPath(req *cmds.Request) (string, error) {
	repoOpt, found := req.Options["config"].(string)
	if found && repoOpt != "" {
		return repoOpt, nil
	}

	repoPath, err := fsrepo.BestKnownPath()
	if err != nil {
		return "", err
	}
	return repoPath, nil
}
