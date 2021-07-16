package migrations

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	config "github.com/ipfs/go-ipfs-config"
)

func TestFindMigrations(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "migratetest")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	migs, bins, err := findMigrations(ctx, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 5 {
		t.Fatal("expected 5 migrations")
	}
	if len(bins) != 0 {
		t.Fatal("should not have found migrations")
	}

	for i := 1; i < 6; i++ {
		createFakeBin(i-1, i, tmpDir)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir)
	defer os.Setenv("PATH", origPath)

	migs, bins, err = findMigrations(ctx, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 5 {
		t.Fatal("expected 5 migrations")
	}
	if len(bins) != len(migs) {
		t.Fatal("missing", len(migs)-len(bins), "migrations")
	}

	os.Remove(bins[migs[2]])

	migs, bins, err = findMigrations(ctx, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(bins) != len(migs)-1 {
		t.Fatal("should be missing one migration bin")
	}
}

func TestFindMigrationsReverse(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "migratetest")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	migs, bins, err := findMigrations(ctx, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 5 {
		t.Fatal("expected 5 migrations")
	}
	if len(bins) != 0 {
		t.Fatal("should not have found migrations")
	}

	for i := 1; i < 6; i++ {
		createFakeBin(i-1, i, tmpDir)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir)
	defer os.Setenv("PATH", origPath)

	migs, bins, err = findMigrations(ctx, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 5 {
		t.Fatal("expected 5 migrations")
	}
	if len(bins) != len(migs) {
		t.Fatal("missing", len(migs)-len(bins), "migrations:", migs)
	}

	os.Remove(bins[migs[2]])

	migs, bins, err = findMigrations(ctx, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(bins) != len(migs)-1 {
		t.Fatal("should be missing one migration bin")
	}
}

func TestFetchMigrations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := createTestServer()
	defer ts.Close()
	fetcher := NewHttpFetcher(CurrentIpfsDist, ts.URL, "", 0)

	tmpDir, err := ioutil.TempDir("", "migratetest")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	needed := []string{"fs-repo-1-to-2", "fs-repo-2-to-3"}
	buf := new(strings.Builder)
	buf.Grow(256)
	logger := log.New(buf, "", 0)
	fetched, err := fetchMigrations(ctx, fetcher, needed, tmpDir, logger)
	if err != nil {
		t.Fatal(err)
	}

	for _, bin := range fetched {
		_, err = os.Stat(bin)
		if os.IsNotExist(err) {
			t.Error("expected file to exist:", bin)
		}
	}

	// Check expected log output
	for _, mig := range needed {
		logOut := fmt.Sprintf("Downloading migration: %s", mig)
		if !strings.Contains(buf.String(), logOut) {
			t.Fatalf("did not find expected log output %q", logOut)
		}
		logOut = fmt.Sprintf("Downloaded and unpacked migration: %s", filepath.Join(tmpDir, mig))
		if !strings.Contains(buf.String(), logOut) {
			t.Fatalf("did not find expected log output %q", logOut)
		}
	}
}

func TestRunMigrations(t *testing.T) {
	fakeHome, err := ioutil.TempDir("", "testhome")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(fakeHome)

	os.Setenv("HOME", fakeHome)
	fakeIpfs := filepath.Join(fakeHome, ".ipfs")

	err = os.Mkdir(fakeIpfs, os.ModePerm)
	if err != nil {
		panic(err)
	}

	testVer := 11
	err = WriteRepoVersion(fakeIpfs, testVer)
	if err != nil {
		t.Fatal(err)
	}

	ts := createTestServer()
	defer ts.Close()
	fetcher := NewHttpFetcher(CurrentIpfsDist, ts.URL, "", 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	targetVer := 9

	err = RunMigration(ctx, fetcher, targetVer, fakeIpfs, false)
	if err == nil || !strings.HasPrefix(err.Error(), "downgrade not allowed") {
		t.Fatal("expected 'downgrade not alloed' error")
	}

	err = RunMigration(ctx, fetcher, targetVer, fakeIpfs, true)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "migration fs-repo-10-to-11 failed") {
			t.Fatal(err)
		}
	}
}

func createFakeBin(from, to int, tmpDir string) {
	migPath := filepath.Join(tmpDir, ExeName(migrationName(from, to)))
	emptyFile, err := os.Create(migPath)
	if err != nil {
		panic(err)
	}
	emptyFile.Close()
	err = os.Chmod(migPath, 0755)
	if err != nil {
		panic(err)
	}
}

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

func TestReadMigrationConfigDefaults(t *testing.T) {
	tmpDir := makeConfig("{}")
	defer os.RemoveAll(tmpDir)

	cfg, err := ReadMigrationConfig(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Keep != config.DefaultMigrationKeep {
		t.Error("expected default value for Keep")
	}

	if len(cfg.DownloadSources) != len(config.DefaultMigrationDownloadSources) {
		t.Fatal("expected default number of download sources")
	}
	for i, src := range config.DefaultMigrationDownloadSources {
		if cfg.DownloadSources[i] != src {
			t.Errorf("wrong DownloadSource: %s", cfg.DownloadSources[i])
		}
	}
}

func TestReadMigrationConfigErrors(t *testing.T) {
	tmpDir := makeConfig(`{"Migration": {"Keep": "badvalue"}}`)
	defer os.RemoveAll(tmpDir)

	_, err := ReadMigrationConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.HasPrefix(err.Error(), "unknown") {
		t.Fatal("did not get expected error:", err)
	}

	os.RemoveAll(tmpDir)
	_, err = ReadMigrationConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error")
	}

	tmpDir = makeConfig(`}{`)
	defer os.RemoveAll(tmpDir)
	_, err = ReadMigrationConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadMigrationConfig(t *testing.T) {
	tmpDir := makeConfig(testConfig)
	defer os.RemoveAll(tmpDir)

	cfg, err := ReadMigrationConfig(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.DownloadSources) != 4 {
		t.Fatal("wrong number of DownloadSources")
	}
	expect := []string{"IPFS", "HTTP", "127.0.0.1", "https://127.0.1.1"}
	for i := range expect {
		if cfg.DownloadSources[i] != expect[i] {
			t.Errorf("wrong DownloadSource at %d", i)
		}
	}

	if cfg.Keep != "cache" {
		t.Error("wrong value for Keep")
	}
}

type mockIpfsFetcher struct{}

func (m *mockIpfsFetcher) Fetch(ctx context.Context, filePath string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockIpfsFetcher) Close() error {
	return nil
}

func TestGetMigrationFetcher(t *testing.T) {
	var f Fetcher
	var err error

	newIpfsFetcher := func(distPath string) Fetcher {
		return &mockIpfsFetcher{}
	}

	downloadSources := []string{"ftp://bad.gateway.io"}
	_, err = GetMigrationFetcher(downloadSources, "", newIpfsFetcher)
	if err == nil || !strings.HasPrefix(err.Error(), "bad gateway addr") {
		t.Fatal("Expected bad gateway address error, got:", err)
	}

	downloadSources = []string{"::bad.gateway.io"}
	_, err = GetMigrationFetcher(downloadSources, "", newIpfsFetcher)
	if err == nil || !strings.HasPrefix(err.Error(), "bad gateway addr") {
		t.Fatal("Expected bad gateway address error, got:", err)
	}

	downloadSources = []string{"http://localhost"}
	f, err = GetMigrationFetcher(downloadSources, "", newIpfsFetcher)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.(*HttpFetcher); !ok {
		t.Fatal("expected HttpFetcher")
	}

	downloadSources = []string{"ipfs"}
	f, err = GetMigrationFetcher(downloadSources, "", newIpfsFetcher)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.(*mockIpfsFetcher); !ok {
		t.Fatal("expected IpfsFetcher")
	}

	downloadSources = []string{"http"}
	f, err = GetMigrationFetcher(downloadSources, "", newIpfsFetcher)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.(*HttpFetcher); !ok {
		t.Fatal("expected HttpFetcher")
	}

	downloadSources = []string{"IPFS", "HTTPS"}
	f, err = GetMigrationFetcher(downloadSources, "", newIpfsFetcher)
	if err != nil {
		t.Fatal(err)
	}
	mf, ok := f.(*MultiFetcher)
	if !ok {
		t.Fatal("expected MultiFetcher")
	}
	if mf.Len() != 2 {
		t.Fatal("expected 2 fetchers in MultiFetcher")
	}

	downloadSources = []string{"ipfs", "https", "some.domain.io"}
	f, err = GetMigrationFetcher(downloadSources, "", newIpfsFetcher)
	if err != nil {
		t.Fatal(err)
	}
	mf, ok = f.(*MultiFetcher)
	if !ok {
		t.Fatal("expected MultiFetcher")
	}
	if mf.Len() != 3 {
		t.Fatal("expected 3 fetchers in MultiFetcher")
	}

	downloadSources = nil
	_, err = GetMigrationFetcher(downloadSources, "", newIpfsFetcher)
	if err == nil {
		t.Fatal("expected error when no sources specified")
	}

	downloadSources = []string{"", ""}
	_, err = GetMigrationFetcher(downloadSources, "", newIpfsFetcher)
	if err == nil {
		t.Fatal("expected error when empty string fetchers specified")
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
