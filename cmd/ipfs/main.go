// cmd/ipfs implements the primary CLI binary for ipfs
package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"runtime/pprof"
	"time"

	util "github.com/ipfs/go-ipfs/cmd/ipfs/util"
	oldcmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	corecmds "github.com/ipfs/go-ipfs/core/commands"
	corehttp "github.com/ipfs/go-ipfs/core/corehttp"
	loader "github.com/ipfs/go-ipfs/plugin/loader"
	repo "github.com/ipfs/go-ipfs/repo"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cli"
	cmdhttp "github.com/ipfs/go-ipfs-cmds/http"
	config "github.com/ipfs/go-ipfs-config"
	u "github.com/ipfs/go-ipfs-util"
	logging "github.com/ipfs/go-log"
	loggables "github.com/libp2p/go-libp2p-loggables"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	manet "github.com/multiformats/go-multiaddr/net"
)

// log is the command logger
var log = logging.Logger("cmd/ipfs")

// declared as a var for testing purposes
var dnsResolver = madns.DefaultResolver

const (
	EnvEnableProfiling = "IPFS_PROF"
	cpuProfile         = "ipfs.cpuprof"
	heapProfile        = "ipfs.memprof"
)

func loadPlugins(repoPath string) (*loader.PluginLoader, error) {
	plugins, err := loader.NewPluginLoader(repoPath)
	if err != nil {
		return nil, fmt.Errorf("error loading plugins: %s", err)
	}

	if err := plugins.Initialize(); err != nil {
		return nil, fmt.Errorf("error initializing plugins: %s", err)
	}

	if err := plugins.Inject(); err != nil {
		return nil, fmt.Errorf("error initializing plugins: %s", err)
	}
	return plugins, nil
}

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

	// we'll call this local helper to output errors.
	// this is so we control how to print errors in one place.
	printErr := func(err error) {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	}

	stopFunc, err := profileIfEnabled()
	if err != nil {
		printErr(err)
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

		//Handle `ipfs help` and `ipfs help <sub-command>`
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
	} else if insideGUI() { // if no args were passed, and we're in a GUI environment
		// launch the daemon instead of launching a ghost window
		os.Args = append(os.Args, "daemon", "--init")
	}

	// output depends on executable name passed in os.Args
	// so we need to make sure it's stable
	os.Args[0] = "ipfs"

	var pluginLoader *loader.PluginLoader
	if req, errParse := cli.Parse(ctx, os.Args[1:], nil, Root); errParse == nil {
		if repoPath, err := getRepoPath(req); err == nil {
			pluginLoader, err = loadPlugins(repoPath)
		}
	}

	buildEnv := func(ctx context.Context, req *cmds.Request) (cmds.Environment, error) {
		checkDebug(req)
		repoPath, err := getRepoPath(req)
		if err != nil {
			return nil, err
		}
		log.Debugf("config path is %s", repoPath)

		// If plugins failed to load before parsing stdin / args fully, defer that until now.
		if pluginLoader == nil {
			pluginLoader, err = loadPlugins(repoPath)
			if err != nil {
				return nil, err
			}
		}

		// this sets up the function that will initialize the node
		// this is so that we can construct the node lazily.
		return &oldcmds.Context{
			ConfigRoot: repoPath,
			LoadConfig: loadConfig,
			ReqLog:     &oldcmds.ReqLog{},
			Plugins:    pluginLoader,
			ConstructNode: func() (n *core.IpfsNode, err error) {
				if req == nil {
					return nil, errors.New("constructing node without a request")
				}

				r, err := fsrepo.Open(repoPath)
				if err != nil { // repo is owned by the node
					return nil, err
				}

				// ok everything is good. set it on the invocation (for ownership)
				// and return it.
				n, err = core.NewNode(ctx, &core.BuildCfg{
					Repo: r,
				})
				if err != nil {
					return nil, err
				}

				return n, nil
			},
		}, nil
	}

	err = cli.Run(ctx, getRoot(), os.Args, os.Stdin, os.Stdout, os.Stderr, buildEnv, makeExecutor)
	if err != nil {
		return 1
	}

	// everything went better than expected :)
	return 0
}

func insideGUI() bool {
	return util.InsideGUI()
}

func checkDebug(req *cmds.Request) {
	// check if user wants to debug. option OR env var.
	debug, _ := req.Options["debug"].(bool)
	if debug || os.Getenv("IPFS_LOGGING") == "debug" {
		u.Debug = true
		logging.SetDebugLogging()
	}
	if u.GetenvBool("DEBUG") {
		u.Debug = true
	}
}

func apiAddrOption(req *cmds.Request) (ma.Multiaddr, error) {
	apiAddrStr, apiSpecified := req.Options[corecmds.ApiOption].(string)
	if !apiSpecified {
		return nil, nil
	}
	return ma.NewMultiaddr(apiAddrStr)
}

func makeExecutor(req *cmds.Request, env interface{}) (cmds.Executor, error) {
	exe := cmds.NewExecutor(req.Root)
	cctx := env.(*oldcmds.Context)

	// Check if the command is disabled.
	if req.Command.NoLocal && req.Command.NoRemote {
		return nil, fmt.Errorf("command disabled: %v", req.Path)
	}

	// Can we just run this locally?
	if !req.Command.NoLocal {
		if doesNotUseRepo, ok := corecmds.GetDoesNotUseRepo(req.Command.Extra); doesNotUseRepo && ok {
			return exe, nil
		}
	}

	// Get the API option from the commandline.
	apiAddr, err := apiAddrOption(req)
	if err != nil {
		return nil, err
	}

	// Require that the command be run on the daemon when the API flag is
	// passed (unless we're trying to _run_ the daemon).
	daemonRequested := apiAddr != nil && req.Command != daemonCmd

	// Run this on the client if required.
	if req.Command.NoRemote {
		if daemonRequested {
			// User requested that the command be run on the daemon but we can't.
			// NOTE: We drop this check for the `ipfs daemon` command.
			return nil, errors.New("api flag specified but command cannot be run on the daemon")
		}
		return exe, nil
	}

	// Finally, look in the repo for an API file.
	if apiAddr == nil {
		var err error
		apiAddr, err = fsrepo.APIAddr(cctx.ConfigRoot)
		switch err {
		case nil, repo.ErrApiNotRunning:
		default:
			return nil, err
		}
	}

	// Still no api specified? Run it on the client or fail.
	if apiAddr == nil {
		if req.Command.NoLocal {
			return nil, fmt.Errorf("command must be run on the daemon: %v", req.Path)
		}
		return exe, nil
	}

	// Resolve the API addr.
	apiAddr, err = resolveAddr(req.Context, apiAddr)
	if err != nil {
		return nil, err
	}
	network, host, err := manet.DialArgs(apiAddr)
	if err != nil {
		return nil, err
	}

	// Construct the executor.
	opts := []cmdhttp.ClientOpt{
		cmdhttp.ClientWithAPIPrefix(corehttp.APIPath),
	}

	// Fallback on a local executor if we (a) have a repo and (b) aren't
	// forcing a daemon.
	if !daemonRequested && fsrepo.IsInitialized(cctx.ConfigRoot) {
		opts = append(opts, cmdhttp.ClientWithFallback(exe))
	}

	switch network {
	case "tcp", "tcp4", "tcp6":
	case "unix":
		path := host
		host = "unix"
		opts = append(opts, cmdhttp.ClientWithHTTPClient(&http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", path)
				},
			},
		}))
	default:
		return nil, fmt.Errorf("unsupported API address: %s", apiAddr)
	}

	return cmdhttp.NewClient(host, opts...), nil
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

func loadConfig(path string) (*config.Config, error) {
	return fsrepo.ConfigAt(path)
}

// startProfiling begins CPU profiling and returns a `stop` function to be
// executed as late as possible. The stop function captures the memprofile.
func startProfiling() (func(), error) {
	// start CPU profiling as early as possible
	ofi, err := os.Create(cpuProfile)
	if err != nil {
		return nil, err
	}
	err = pprof.StartCPUProfile(ofi)
	if err != nil {
		ofi.Close()
		return nil, err
	}
	go func() {
		for range time.NewTicker(time.Second * 30).C {
			err := writeHeapProfileToFile()
			if err != nil {
				log.Error(err)
			}
		}
	}()

	stopProfiling := func() {
		pprof.StopCPUProfile()
		ofi.Close() // captured by the closure
	}
	return stopProfiling, nil
}

func writeHeapProfileToFile() error {
	mprof, err := os.Create(heapProfile)
	if err != nil {
		return err
	}
	defer mprof.Close() // _after_ writing the heap profile
	return pprof.WriteHeapProfile(mprof)
}

func profileIfEnabled() (func(), error) {
	// FIXME this is a temporary hack so profiling of asynchronous operations
	// works as intended.
	if os.Getenv(EnvEnableProfiling) != "" {
		stopProfilingFunc, err := startProfiling() // TODO maybe change this to its own option... profiling makes it slower.
		if err != nil {
			return nil, err
		}
		return stopProfilingFunc, nil
	}
	return func() {}, nil
}

func resolveAddr(ctx context.Context, addr ma.Multiaddr) (ma.Multiaddr, error) {
	ctx, cancelFunc := context.WithTimeout(ctx, 10*time.Second)
	defer cancelFunc()

	addrs, err := dnsResolver.Resolve(ctx, addr)
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		return nil, errors.New("non-resolvable API endpoint")
	}

	return addrs[0], nil
}
