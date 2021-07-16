package ipfsfetcher

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
)

func init() {
	err := setupPlugins()
	if err != nil {
		panic(err)
	}
}

func TestIpfsFetcher(t *testing.T) {
	skipUnlessEpic(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := NewIpfsFetcher("", 0, nil)
	defer fetcher.Close()

	rc, err := fetcher.Fetch(ctx, "go-ipfs/versions")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	var lines []string
	scan := bufio.NewScanner(rc)
	for scan.Scan() {
		lines = append(lines, scan.Text())
	}
	err = scan.Err()
	if err != nil {
		t.Fatal("could not read versions:", err)
	}

	if len(lines) < 6 {
		t.Fatal("do not get all expected data")
	}
	if lines[0] != "v0.3.2" {
		t.Fatal("expected v1.0.0 as first line, got", lines[0])
	}

	// Check not found
	_, err = fetcher.Fetch(ctx, "/no_such_file")
	if err == nil {
		t.Fatal("expected error 404")
	}

}

func TestInitIpfsFetcher(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f := NewIpfsFetcher("", 0, nil)
	defer f.Close()

	// Init ipfs repo
	f.ipfsTmpDir, f.openErr = initTempNode(ctx, nil, nil)
	if f.openErr != nil {
		t.Fatalf("failed to initialize ipfs node: %s", f.openErr)
	}

	// Start ipfs node
	f.openErr = f.startTempNode(ctx)
	if f.openErr != nil {
		t.Errorf("failed to start ipfs node: %s", f.openErr)
		return
	}

	var stopFuncCalled bool
	stopFunc := f.ipfsStopFunc
	f.ipfsStopFunc = func() {
		stopFuncCalled = true
		stopFunc()
	}

	addrInfo := f.AddrInfo()
	if string(addrInfo.ID) == "" {
		t.Error("AddInfo ID not set")
	}
	if len(addrInfo.Addrs) == 0 {
		t.Error("AddInfo Addrs not set")
	}
	t.Log("Temp node listening on:", addrInfo.Addrs)

	err := f.Close()
	if err != nil {
		t.Fatalf("failed to close fetcher: %s", err)
	}

	if stopFunc != nil && !stopFuncCalled {
		t.Error("Close did not call stop function")
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("failed to close fetcher 2nd time: %s", err)
	}
}

func TestReadIpfsConfig(t *testing.T) {
	var testConfig = `
{
	"Bootstrap": [
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
	],
	"Migration": {
		"DownloadSources": ["IPFS", "HTTP", "127.0.0.1", "https://127.0.1.1"],
		"Keep": "cache"
	},
	"Peering": {
		"Peers": [
			{
				"ID": "12D3KooWGC6TvWhfapngX6wvJHMYvKpDMXPb3ZnCZ6dMoaMtimQ5",
				"Addrs": ["/ip4/127.0.0.1/tcp/4001", "/ip4/127.0.0.1/udp/4001/quic"]
			}
		]
	}
}
`

	noSuchDir := "no_such_dir-5953aa51-1145-4efd-afd1-a069075fcf76"
	bootstrap, peers := readIpfsConfig(&noSuchDir)
	if bootstrap != nil {
		t.Error("expected nil bootstrap")
	}
	if peers != nil {
		t.Error("expected nil peers")
	}

	tmpDir := makeConfig(testConfig)
	defer os.RemoveAll(tmpDir)

	bootstrap, peers = readIpfsConfig(nil)
	if bootstrap != nil || peers != nil {
		t.Fatal("expected nil ipfs config items")
	}

	bootstrap, peers = readIpfsConfig(&tmpDir)
	if len(bootstrap) != 2 {
		t.Fatal("wrong number of bootstrap addresses")
	}
	if bootstrap[0] != "/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt" {
		t.Fatal("wrong bootstrap address")
	}

	if len(peers) != 1 {
		t.Fatal("wrong number of peers")
	}

	peer := peers[0]
	if peer.ID.String() != "12D3KooWGC6TvWhfapngX6wvJHMYvKpDMXPb3ZnCZ6dMoaMtimQ5" {
		t.Errorf("wrong ID for first peer")
	}
	if len(peer.Addrs) != 2 {
		t.Error("wrong number of addrs for first peer")
	}
}

func TestReadPartialIpfsConfig(t *testing.T) {
	const (
		configBadBootstrap = `
{
	"Bootstrap": "unreadable",
	"Migration": {
		"DownloadSources": ["IPFS", "HTTP", "127.0.0.1"],
		"Keep": "cache"
	},
	"Peering": {
		"Peers": [
			{
				"ID": "12D3KooWGC6TvWhfapngX6wvJHMYvKpDMXPb3ZnCZ6dMoaMtimQ5",
				"Addrs": ["/ip4/127.0.0.1/tcp/4001", "/ip4/127.0.0.1/udp/4001/quic"]
			}
		]
	}
}
`
		configBadPeers = `
{
	"Bootstrap": [
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
	],
	"Migration": {
		"DownloadSources": ["IPFS", "HTTP", "127.0.0.1"],
		"Keep": "cache"
	},
	"Peering": "Unreadable-data"
}
`
	)

	tmpDir := makeConfig(configBadBootstrap)
	defer os.RemoveAll(tmpDir)

	bootstrap, peers := readIpfsConfig(&tmpDir)
	if bootstrap != nil {
		t.Fatal("expected nil bootstrap")
	}
	if len(peers) != 1 {
		t.Fatal("wrong number of peers")
	}
	if len(peers[0].Addrs) != 2 {
		t.Error("wrong number of addrs for first peer")
	}
	os.RemoveAll(tmpDir)

	tmpDir = makeConfig(configBadPeers)
	defer os.RemoveAll(tmpDir)

	bootstrap, peers = readIpfsConfig(&tmpDir)
	if peers != nil {
		t.Fatal("expected nil peers")
	}
	if len(bootstrap) != 2 {
		t.Fatal("wrong number of bootstrap addresses")
	}
	if bootstrap[0] != "/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt" {
		t.Fatal("wrong bootstrap address")
	}
}

func makeConfig(configData string) string {
	tmpDir, err := ioutil.TempDir("", "migration_test")
	if err != nil {
		panic(err)
	}

	cfgFile, err := os.Create(filepath.Join(tmpDir, "config"))
	if err != nil {
		panic(err)
	}
	if _, err = cfgFile.Write([]byte(configData)); err != nil {
		panic(err)
	}
	if err = cfgFile.Close(); err != nil {
		panic(err)
	}
	return tmpDir
}

func skipUnlessEpic(t *testing.T) {
	if os.Getenv("IPFS_EPIC_TEST") == "" {
		t.SkipNow()
	}
}

func setupPlugins() error {
	defaultPath, err := migrations.IpfsDir("")
	if err != nil {
		return err
	}

	// Load plugins. This will skip the repo if not available.
	plugins, err := loader.NewPluginLoader(filepath.Join(defaultPath, "plugins"))
	if err != nil {
		return fmt.Errorf("error loading plugins: %w", err)
	}

	if err := plugins.Initialize(); err != nil {
		// Need to ignore errors here because plugins may already be loaded when
		// run from ipfs daemon.
		return fmt.Errorf("error initializing plugins: %w", err)
	}

	if err := plugins.Inject(); err != nil {
		// Need to ignore errors here because plugins may already be loaded when
		// run from ipfs daemon.
		return fmt.Errorf("error injecting plugins: %w", err)
	}

	return nil
}
