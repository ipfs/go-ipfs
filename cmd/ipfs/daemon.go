package main

import (
	_ "expvar"
	"fmt"
	_ "net/http/pprof"
	"os"
	"runtime"
	"sort"
	"sync"

	version "github.com/ipfs/go-ipfs"
	utilmain "github.com/ipfs/go-ipfs/cmd/ipfs/util"
	oldcmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	commands "github.com/ipfs/go-ipfs/core/commands"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	corehttp "github.com/ipfs/go-ipfs/core/corehttp"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
	nodeMount "github.com/ipfs/go-ipfs/fuse/node"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"

	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-ipfs-cmds"
	mprome "github.com/ipfs/go-metrics-prometheus"
	goprocess "github.com/jbenet/goprocess"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multiaddr-net"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	adjustFDLimitKwd         = "manage-fdlimit"
	enableGCKwd              = "enable-gc"
	initOptionKwd            = "init"
	initProfileOptionKwd     = "init-profile"
	ipfsMountKwd             = "mount-ipfs"
	ipnsMountKwd             = "mount-ipns"
	migrateKwd               = "migrate"
	mountKwd                 = "mount"
	offlineKwd               = "offline" // global option
	routingOptionKwd         = "routing"
	unencryptTransportKwd    = "disable-transport-encryption"
	unrestrictedApiAccessKwd = "unrestricted-api"
	writableKwd              = "writable"
	enablePubSubKwd          = "enable-pubsub-experiment"
	enableIPNSPubSubKwd      = "enable-namesys-pubsub"
	enableMultiplexKwd       = "enable-mplex-experiment"

	routingOptionDHTClientKwd = "dhtclient"
	routingOptionDHTKwd       = "dht"
	routingOptionNoneKwd      = "none"
	routingOptionDefaultKwd   = "default"
)

var daemonCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Run a network-connected IPFS node.",
		ShortDescription: `
'ipfs daemon' runs a persistent ipfs daemon that can serve commands
over the network. Most applications that use IPFS will do so by
communicating with a daemon over the HTTP API. While the daemon is
running, calls to 'ipfs' commands will be sent over the network to
the daemon.
`,
		LongDescription: `
The daemon will start listening on ports on the network, which are
documented in (and can be modified through) 'ipfs config Addresses'.
For example, to change the 'Gateway' port:

  ipfs config Addresses.Gateway /ip4/127.0.0.1/tcp/8082

The API address can be changed the same way:

  ipfs config Addresses.API /ip4/127.0.0.1/tcp/5002

Make sure to restart the daemon after changing addresses.

By default, the gateway is only accessible locally. To expose it to
other computers in the network, use 0.0.0.0 as the ip address:

  ipfs config Addresses.Gateway /ip4/0.0.0.0/tcp/8080

Be careful if you expose the API. It is a security risk, as anyone could
control your node remotely. If you need to control the node remotely,
make sure to protect the port as you would other services or database
(firewall, authenticated proxy, etc).

HTTP Headers

ipfs supports passing arbitrary headers to the API and Gateway. You can
do this by setting headers on the API.HTTPHeaders and Gateway.HTTPHeaders
keys:

  ipfs config --json API.HTTPHeaders.X-Special-Header "[\"so special :)\"]"
  ipfs config --json Gateway.HTTPHeaders.X-Special-Header "[\"so special :)\"]"

Note that the value of the keys is an _array_ of strings. This is because
headers can have more than one value, and it is convenient to pass through
to other libraries.

CORS Headers (for API)

You can setup CORS headers the same way:

  ipfs config --json API.HTTPHeaders.Access-Control-Allow-Origin "[\"example.com\"]"
  ipfs config --json API.HTTPHeaders.Access-Control-Allow-Methods "[\"PUT\", \"GET\", \"POST\"]"
  ipfs config --json API.HTTPHeaders.Access-Control-Allow-Credentials "[\"true\"]"

Shutdown

To shutdown the daemon, send a SIGINT signal to it (e.g. by pressing 'Ctrl-C')
or send a SIGTERM signal to it (e.g. with 'kill'). It may take a while for the
daemon to shutdown gracefully, but it can be killed forcibly by sending a
second signal.

IPFS_PATH environment variable

ipfs uses a repository in the local file system. By default, the repo is
located at ~/.ipfs. To change the repo location, set the $IPFS_PATH
environment variable:

  export IPFS_PATH=/path/to/ipfsrepo

Routing

IPFS by default will use a DHT for content routing. There is a highly
experimental alternative that operates the DHT in a 'client only' mode that
can be enabled by running the daemon as:

  ipfs daemon --routing=dhtclient

This will later be transitioned into a config option once it gets out of the
'experimental' stage.

DEPRECATION NOTICE

Previously, ipfs used an environment variable as seen below:

  export API_ORIGIN="http://localhost:8888/"

This is deprecated. It is still honored in this version, but will be removed
in a future version, along with this notice. Please move to setting the HTTP
Headers.
`,
	},

	Options: []cmds.Option{
		cmds.BoolOption(initOptionKwd, "Initialize ipfs with default settings if not already initialized"),
		cmds.StringOption(initProfileOptionKwd, "Configuration profiles to apply for --init. See ipfs init --help for more"),
		cmds.StringOption(routingOptionKwd, "Overrides the routing option").WithDefault(routingOptionDefaultKwd),
		cmds.BoolOption(mountKwd, "Mounts IPFS to the filesystem"),
		cmds.BoolOption(writableKwd, "Enable writing objects (with POST, PUT and DELETE)"),
		cmds.StringOption(ipfsMountKwd, "Path to the mountpoint for IPFS (if using --mount). Defaults to config setting."),
		cmds.StringOption(ipnsMountKwd, "Path to the mountpoint for IPNS (if using --mount). Defaults to config setting."),
		cmds.BoolOption(unrestrictedApiAccessKwd, "Allow API access to unlisted hashes"),
		cmds.BoolOption(unencryptTransportKwd, "Disable transport encryption (for debugging protocols)"),
		cmds.BoolOption(enableGCKwd, "Enable automatic periodic repo garbage collection"),
		cmds.BoolOption(adjustFDLimitKwd, "Check and raise file descriptor limits if needed").WithDefault(true),
		cmds.BoolOption(migrateKwd, "If true, assume yes at the migrate prompt. If false, assume no."),
		cmds.BoolOption(enablePubSubKwd, "Instantiate the ipfs daemon with the experimental pubsub feature enabled."),
		cmds.BoolOption(enableIPNSPubSubKwd, "Enable IPNS record distribution through pubsub; enables pubsub."),
		cmds.BoolOption(enableMultiplexKwd, "Add the experimental 'go-multiplex' stream muxer to libp2p on construction.").WithDefault(true),
	},
	Subcommands: map[string]*cmds.Command{},
	Run:         daemonFunc,
}

func daemonFunc(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) (_err error) {
	// Inject metrics before we do anything
	err := mprome.Inject()
	if err != nil {
		log.Errorf("Injecting prometheus handler for metrics failed with message: %s\n", err.Error())
	}

	// let the user know we're going.
	fmt.Printf("Initializing daemon...\n")

	defer func() {
		if _err != nil {
			// Print an extra line before any errors. This could go
			// in the commands lib but doesn't really make sense for
			// all commands.
			fmt.Println()
		}
	}()

	// print the ipfs version
	printVersion()

	managefd, _ := req.Options[adjustFDLimitKwd].(bool)
	if managefd {
		if _, _, err := utilmain.ManageFdLimit(); err != nil {
			log.Errorf("setting file descriptor limit: %s", err)
		}
	}

	cctx := env.(*oldcmds.Context)

	// first, whether user has provided the initialization flag. we may be
	// running in an uninitialized state.
	initialize, _ := req.Options[initOptionKwd].(bool)
	if initialize {
		if !fsrepo.IsInitialized(cctx.RepoPath) {
			profiles, _ := req.Options[initProfileOptionKwd].(string)

			err := initWithDefaults(os.Stdout, cctx.RepoPath, profiles)
			if err != nil {
				return err
			}
		}
	}

	builder := &nodeBuilder{
		ctx:      req.Context,
		repoPath: cctx.RepoPath,

		permanent: true,
		daemon:    true,
	}

	builder.unencrypted, _ = req.Options[unencryptTransportKwd].(bool)
	builder.migrate, builder.userMigrate = req.Options[migrateKwd].(bool)

	builder.routing, _ = req.Options[routingOptionKwd].(string)

	cfg, err := cctx.GetConfig()
	if err != nil {
		return err
	}

	builder.offline, _ = req.Options[offlineKwd].(bool)
	builder.ipnsps, _ = req.Options[enableIPNSPubSubKwd].(bool)
	builder.pubsub, _ = req.Options[enablePubSubKwd].(bool)
	builder.mplex, _ = req.Options[enableMultiplexKwd].(bool)

	node, err := builder.buildNode()
	if err != nil {
		return err
	}

	if node.PNetFingerprint != nil {
		fmt.Println("Swarm is limited to private network of peers with the swarm key")
		fmt.Printf("Swarm key fingerprint: %x\n", node.PNetFingerprint)
	}

	printSwarmAddrs(node)

	defer func() {
		// We wait for the node to close first, as the node has children
		// that it will wait for before closing, such as the API server.
		node.Close() //TODO: errors?

		select {
		case <-req.Context.Done():
			log.Info("Gracefully shut down daemon")
		default:
		}
	}()

	cctx.ConstructNode = func() (*core.IpfsNode, error) {
		return node, nil
	}

	// Start "core" plugins. We want to do this *before* starting the HTTP
	// API as the user may be relying on these plugins.
	api, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return err
	}
	err = cctx.Plugins.Start(api)
	if err != nil {
		return err
	}
	node.Process.AddChild(goprocess.WithTeardown(cctx.Plugins.Close))

	// construct api endpoint - every time
	apiErrc, err := serveHTTPApi(req, cctx)
	if err != nil {
		return err
	}

	// construct fuse mountpoints - if the user provided the --mount flag
	mount, _ := req.Options[mountKwd].(bool)
	if mount && builder.offline {
		return cmds.Errorf(cmds.ErrClient, "mount is not currently supported in offline mode")
	}
	if mount {
		if err := mountFuse(req, cctx); err != nil {
			return err
		}
	}

	// repo blockstore GC - if --enable-gc flag is present
	gcErrc, err := maybeRunGC(req, node)
	if err != nil {
		return err
	}

	// construct http gateway - if it is set in the config
	var gwErrc <-chan error
	if len(cfg.Addresses.Gateway) > 0 {
		var err error
		gwErrc, err = serveHTTPGateway(req, cctx)
		if err != nil {
			return err
		}
	}

	// initialize metrics collector
	prometheus.MustRegister(&corehttp.IpfsNodeCollector{Node: node})

	// The daemon is *finally* ready.
	fmt.Printf("Daemon is ready\n")

	// Give the user some immediate feedback when they hit C-c
	go func() {
		<-req.Context.Done()
		fmt.Println("Received interrupt signal, shutting down...")
		fmt.Println("(Hit ctrl-c again to force-shutdown the daemon.)")
	}()

	// collect long-running errors and block for shutdown
	// TODO(cryptix): our fuse currently doesnt follow this pattern for graceful shutdown
	var errs error
	for err := range merge(apiErrc, gwErrc, gcErrc) {
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

// serveHTTPApi collects options, creates listener, prints status message and starts serving requests
func serveHTTPApi(req *cmds.Request, cctx *oldcmds.Context) (<-chan error, error) {
	cfg, err := cctx.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: GetConfig() failed: %s", err)
	}

	apiAddrs := make([]string, 0, 2)
	apiAddr, _ := req.Options[commands.ApiOption].(string)
	if apiAddr == "" {
		apiAddrs = cfg.Addresses.API
	} else {
		apiAddrs = append(apiAddrs, apiAddr)
	}

	listeners := make([]manet.Listener, 0, len(apiAddrs))
	for _, addr := range apiAddrs {
		apiMaddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("serveHTTPApi: invalid API address: %q (err: %s)", apiAddr, err)
		}

		apiLis, err := manet.Listen(apiMaddr)
		if err != nil {
			return nil, fmt.Errorf("serveHTTPApi: manet.Listen(%s) failed: %s", apiMaddr, err)
		}

		// we might have listened to /tcp/0 - lets see what we are listing on
		apiMaddr = apiLis.Multiaddr()
		fmt.Printf("API server listening on %s\n", apiMaddr)
		fmt.Printf("WebUI: http://%s/webui\n", apiLis.Addr())
		listeners = append(listeners, apiLis)
	}

	// by default, we don't let you load arbitrary ipfs objects through the api,
	// because this would open up the api to scripting vulnerabilities.
	// only the webui objects are allowed.
	// if you know what you're doing, go ahead and pass --unrestricted-api.
	unrestricted, _ := req.Options[unrestrictedApiAccessKwd].(bool)
	gatewayOpt := corehttp.GatewayOption(false, corehttp.WebUIPaths...)
	if unrestricted {
		gatewayOpt = corehttp.GatewayOption(true, "/ipfs", "/ipns")
	}

	var opts = []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("api"),
		corehttp.CheckVersionOption(),
		corehttp.CommandsOption(*cctx),
		corehttp.WebUIOption,
		gatewayOpt,
		corehttp.VersionOption(),
		corehttp.DefaultMux("/debug/vars"),
		corehttp.DefaultMux("/debug/pprof/"),
		corehttp.MutexFractionOption("/debug/pprof-mutex/"),
		corehttp.MetricsScrapingOption("/debug/metrics/prometheus"),
		corehttp.LogOption(),
	}

	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	node, err := cctx.ConstructNode()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: ConstructNode() failed: %s", err)
	}

	if err := node.Repo.SetAPIAddr(listeners[0].Multiaddr()); err != nil {
		return nil, fmt.Errorf("serveHTTPApi: SetAPIAddr() failed: %s", err)
	}

	errc := make(chan error)
	var wg sync.WaitGroup
	for _, apiLis := range listeners {
		wg.Add(1)
		go func(lis manet.Listener) {
			defer wg.Done()
			errc <- corehttp.Serve(node, manet.NetListener(lis), opts...)
		}(apiLis)
	}

	go func() {
		wg.Wait()
		close(errc)
	}()

	return errc, nil
}

// printSwarmAddrs prints the addresses of the host
func printSwarmAddrs(node *core.IpfsNode) {
	if !node.IsOnline {
		fmt.Println("Swarm not listening, running in offline mode.")
		return
	}

	var lisAddrs []string
	ifaceAddrs, err := node.PeerHost.Network().InterfaceListenAddresses()
	if err != nil {
		log.Errorf("failed to read listening addresses: %s", err)
	}
	for _, addr := range ifaceAddrs {
		lisAddrs = append(lisAddrs, addr.String())
	}
	sort.Strings(lisAddrs)
	for _, addr := range lisAddrs {
		fmt.Printf("Swarm listening on %s\n", addr)
	}

	var addrs []string
	for _, addr := range node.PeerHost.Addrs() {
		addrs = append(addrs, addr.String())
	}
	sort.Strings(addrs)
	for _, addr := range addrs {
		fmt.Printf("Swarm announcing %s\n", addr)
	}

}

// serveHTTPGateway collects options, creates listener, prints status message and starts serving requests
func serveHTTPGateway(req *cmds.Request, cctx *oldcmds.Context) (<-chan error, error) {
	cfg, err := cctx.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPGateway: GetConfig() failed: %s", err)
	}

	writable, writableOptionFound := req.Options[writableKwd].(bool)
	if !writableOptionFound {
		writable = cfg.Gateway.Writable
	}

	gatewayAddrs := cfg.Addresses.Gateway
	listeners := make([]manet.Listener, 0, len(gatewayAddrs))
	for _, addr := range gatewayAddrs {
		gatewayMaddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("serveHTTPGateway: invalid gateway address: %q (err: %s)", addr, err)
		}

		gwLis, err := manet.Listen(gatewayMaddr)
		if err != nil {
			return nil, fmt.Errorf("serveHTTPGateway: manet.Listen(%s) failed: %s", gatewayMaddr, err)
		}
		// we might have listened to /tcp/0 - lets see what we are listing on
		gatewayMaddr = gwLis.Multiaddr()

		if writable {
			fmt.Printf("Gateway (writable) server listening on %s\n", gatewayMaddr)
		} else {
			fmt.Printf("Gateway (readonly) server listening on %s\n", gatewayMaddr)
		}

		listeners = append(listeners, gwLis)
	}

	cmdctx := *cctx
	cmdctx.Gateway = true

	var opts = []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("gateway"),
		corehttp.IPNSHostnameOption(),
		corehttp.GatewayOption(writable, "/ipfs", "/ipns"),
		corehttp.VersionOption(),
		corehttp.CheckVersionOption(),
		corehttp.CommandsROOption(cmdctx),
	}

	if cfg.Experimental.P2pHttpProxy {
		opts = append(opts, corehttp.ProxyOption())
	}

	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	node, err := cctx.ConstructNode()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPGateway: ConstructNode() failed: %s", err)
	}

	errc := make(chan error)
	var wg sync.WaitGroup
	for _, lis := range listeners {
		wg.Add(1)
		go func(lis manet.Listener) {
			defer wg.Done()
			errc <- corehttp.Serve(node, manet.NetListener(lis), opts...)
		}(lis)
	}

	go func() {
		wg.Wait()
		close(errc)
	}()

	return errc, nil
}

//collects options and opens the fuse mountpoint
func mountFuse(req *cmds.Request, cctx *oldcmds.Context) error {
	cfg, err := cctx.GetConfig()
	if err != nil {
		return fmt.Errorf("mountFuse: GetConfig() failed: %s", err)
	}

	fsdir, found := req.Options[ipfsMountKwd].(string)
	if !found {
		fsdir = cfg.Mounts.IPFS
	}

	nsdir, found := req.Options[ipnsMountKwd].(string)
	if !found {
		nsdir = cfg.Mounts.IPNS
	}

	node, err := cctx.ConstructNode()
	if err != nil {
		return fmt.Errorf("mountFuse: ConstructNode() failed: %s", err)
	}

	err = nodeMount.Mount(node, fsdir, nsdir)
	if err != nil {
		return err
	}
	fmt.Printf("IPFS mounted at: %s\n", fsdir)
	fmt.Printf("IPNS mounted at: %s\n", nsdir)
	return nil
}

func maybeRunGC(req *cmds.Request, node *core.IpfsNode) (<-chan error, error) {
	enableGC, _ := req.Options[enableGCKwd].(bool)
	if !enableGC {
		return nil, nil
	}

	errc := make(chan error)
	go func() {
		errc <- corerepo.PeriodicGC(req.Context, node)
		close(errc)
	}()
	return errc, nil
}

// merge does fan-in of multiple read-only error channels
// taken from http://blog.golang.org/pipelines
func merge(cs ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	out := make(chan error)

	// Start an output goroutine for each input channel in cs.  output
	// copies values from c to out until c is closed, then calls wg.Done.
	output := func(c <-chan error) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}
	for _, c := range cs {
		if c != nil {
			wg.Add(1)
			go output(c)
		}
	}

	// Start a goroutine to close out once all the output goroutines are
	// done.  This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func YesNoPrompt(prompt string) bool {
	var s string
	for i := 0; i < 3; i++ {
		fmt.Printf("%s ", prompt)
		fmt.Scanf("%s", &s)
		switch s {
		case "y", "Y":
			return true
		case "n", "N":
			return false
		case "":
			return false
		}
		fmt.Println("Please press either 'y' or 'n'")
	}

	return false
}

func printVersion() {
	fmt.Printf("go-ipfs version: %s-%s\n", version.CurrentVersionNumber, version.CurrentCommit)
	fmt.Printf("Repo version: %d\n", fsrepo.RepoVersion)
	fmt.Printf("System version: %s\n", runtime.GOARCH+"/"+runtime.GOOS)
	fmt.Printf("Golang version: %s\n", runtime.Version())
}
