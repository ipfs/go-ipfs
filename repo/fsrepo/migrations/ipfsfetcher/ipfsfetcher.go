package ipfsfetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

const (
	// Default maximum download size
	defaultFetchLimit = 1024 * 1024 * 512

	tempNodeTcpAddr = "/ip4/127.0.0.1/tcp/0"
)

type IpfsFetcher struct {
	distPath string
	limit    int64
	repoRoot *string

	openOnce  sync.Once
	openErr   error
	closeOnce sync.Once
	closeErr  error

	ipfs         iface.CoreAPI
	ipfsTmpDir   string
	ipfsStopFunc func()

	fetched []ipath.Path
	mutex   sync.Mutex

	addrInfo peer.AddrInfo
}

// NewIpfsFetcher creates a new IpfsFetcher
//
// Specifying "" for distPath sets the default IPNS path.
// Specifying 0 for fetchLimit sets the default, -1 means no limit.
//
// Bootstrap and peer information in read from the IPFS config file in
// repoRoot, unless repoRoot is nil.  If repoRoot is empty (""), then read the
// config from the default IPFS directory.
func NewIpfsFetcher(distPath string, fetchLimit int64, repoRoot *string) *IpfsFetcher {
	f := &IpfsFetcher{
		limit:    defaultFetchLimit,
		distPath: migrations.LatestIpfsDist,
		repoRoot: repoRoot,
	}

	if distPath != "" {
		if !strings.HasPrefix(distPath, "/") {
			distPath = "/" + distPath
		}
		f.distPath = distPath
	}

	if fetchLimit != 0 {
		if fetchLimit < 0 {
			fetchLimit = 0
		}
		f.limit = fetchLimit
	}

	return f
}

// Fetch attempts to fetch the file at the given path, from the distribution
// site configured for this HttpFetcher.  Returns io.ReadCloser on success,
// which caller must close.
func (f *IpfsFetcher) Fetch(ctx context.Context, filePath string) (io.ReadCloser, error) {
	// Initialize and start IPFS node on first call to Fetch, since the fetcher
	// may be created by not used.
	f.openOnce.Do(func() {
		bootstrap, peers := readIpfsConfig(f.repoRoot)
		f.ipfsTmpDir, f.openErr = initTempNode(ctx, bootstrap, peers)
		if f.openErr != nil {
			return
		}

		f.openErr = f.startTempNode(ctx)
	})

	fmt.Printf("Fetching with IPFS: %q\n", filePath)

	if f.openErr != nil {
		return nil, f.openErr
	}

	iPath, err := parsePath(path.Join(f.distPath, filePath))
	if err != nil {
		return nil, err
	}

	nd, err := f.ipfs.Unixfs().Get(ctx, iPath)
	if err != nil {
		return nil, err
	}

	f.recordFetched(iPath)

	fileNode, ok := nd.(files.File)
	if !ok {
		return nil, fmt.Errorf("%q is not a file", filePath)
	}

	if f.limit != 0 {
		return migrations.NewLimitReadCloser(fileNode, f.limit), nil
	}
	return fileNode, nil
}

func (f *IpfsFetcher) Close() error {
	f.closeOnce.Do(func() {
		if f.ipfsStopFunc != nil {
			// Tell ipfs node to stop and wait for it to stop
			f.ipfsStopFunc()
		}

		if f.ipfsTmpDir != "" {
			// Remove the temp ipfs dir
			f.closeErr = os.RemoveAll(f.ipfsTmpDir)
		}
	})
	return f.closeErr
}

func (f *IpfsFetcher) AddrInfo() peer.AddrInfo {
	return f.addrInfo
}

// FetchedPaths returns the IPFS paths of all items fetched by this fetcher
func (f *IpfsFetcher) FetchedPaths() []ipath.Path {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.fetched
}

func (f *IpfsFetcher) recordFetched(fetchedPath ipath.Path) {
	// Mutex protects against update by concurrent calls to Fetch
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.fetched = append(f.fetched, fetchedPath)
}

func initTempNode(ctx context.Context, bootstrap []string, peers []peer.AddrInfo) (string, error) {
	identity, err := config.CreateIdentity(ioutil.Discard, []options.KeyGenerateOption{
		options.Key.Type(options.Ed25519Key),
	})
	if err != nil {
		return "", err
	}
	cfg, err := config.InitWithIdentity(identity)
	if err != nil {
		return "", err
	}

	// create temporary ipfs directory
	dir, err := ioutil.TempDir("", "ipfs-temp")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}

	// configure the temporary node
	cfg.Routing.Type = "dhtclient"

	// Disable listening for inbound connections
	cfg.Addresses.Gateway = []string{}
	cfg.Addresses.API = []string{}
	cfg.Addresses.Swarm = []string{tempNodeTcpAddr}

	if len(bootstrap) != 0 {
		cfg.Bootstrap = bootstrap
	}

	if len(peers) != 0 {
		cfg.Peering.Peers = peers
	}

	// Assumes that repo plugins are already loaded
	err = fsrepo.Init(dir, cfg)
	if err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("failed to initialize ephemeral node: %s", err)
	}

	return dir, nil
}

func (f *IpfsFetcher) startTempNode(ctx context.Context) error {
	// Open the repo
	r, err := fsrepo.Open(f.ipfsTmpDir)
	if err != nil {
		return err
	}

	// Create a new lifetime context that is used to stop the temp ipfs node
	ctxIpfsLife, cancel := context.WithCancel(context.Background())

	// Construct the node
	node, err := core.NewNode(ctxIpfsLife, &core.BuildCfg{
		Online:  true,
		Routing: libp2p.DHTClientOption,
		Repo:    r,
	})
	if err != nil {
		cancel()
		r.Close()
		return err
	}

	ipfs, err := coreapi.NewCoreAPI(node)
	if err != nil {
		cancel()
		return err
	}

	stopFunc := func() {
		// Tell ipfs to stop
		cancel()
		// Wait until ipfs is stopped
		<-node.Context().Done()

		fmt.Println("migration peer", node.Identity, "shutdown")
	}

	addrs, err := ipfs.Swarm().LocalAddrs(ctx)
	if err != nil {
		// Failure to get the local swarm address only means that the
		// downloaded migrations cannot be fetched through the temporary node.
		// So, print the error message and keep going.
		fmt.Fprintln(os.Stderr, "cannot get local swarm address:", err)
	}

	f.addrInfo = peer.AddrInfo{
		ID:    node.Identity,
		Addrs: addrs,
	}

	f.ipfs = ipfs
	f.ipfsStopFunc = stopFunc

	return nil
}

func parsePath(fetchPath string) (ipath.Path, error) {
	ipfsPath := ipath.New(fetchPath)
	if ipfsPath.IsValid() == nil {
		return ipfsPath, nil
	}

	u, err := url.Parse(fetchPath)
	if err != nil {
		return nil, fmt.Errorf("%q could not be parsed: %s", fetchPath, err)
	}

	switch proto := u.Scheme; proto {
	case "ipfs", "ipld", "ipns":
		ipfsPath = ipath.New(path.Join("/", proto, u.Host, u.Path))
	default:
		return nil, fmt.Errorf("%q is not an IPFS path", fetchPath)
	}
	return ipfsPath, ipfsPath.IsValid()
}

func readIpfsConfig(repoRoot *string) (bootstrap []string, peers []peer.AddrInfo) {
	if repoRoot == nil {
		return
	}

	cfgPath, err := config.Filename(*repoRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer cfgFile.Close()

	// Attempt to read bootstrap addresses
	var bootstrapCfg struct {
		Bootstrap []string
	}
	err = json.NewDecoder(cfgFile).Decode(&bootstrapCfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read bootstrap peers from config")
	} else {
		bootstrap = bootstrapCfg.Bootstrap
	}

	if _, err = cfgFile.Seek(0, 0); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	// Attempt to read peers
	var peeringCfg struct {
		Peering config.Peering
	}
	err = json.NewDecoder(cfgFile).Decode(&peeringCfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read peering from config")
	} else {
		peers = peeringCfg.Peering.Peers
	}

	return
}
