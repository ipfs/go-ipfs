package migrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"sync"

	config "github.com/ipfs/go-ipfs-config"
)

const (
	// Migrations subdirectory in distribution. Empty for root (no subdir).
	distMigsRoot = ""
	distFSRM     = "fs-repo-migrations"
)

// RunMigration finds, downloads, and runs the individual migrations needed to
// migrate the repo from its current version to the target version.
func RunMigration(ctx context.Context, fetcher Fetcher, targetVer int, ipfsDir string, allowDowngrade bool) error {
	ipfsDir, err := CheckIpfsDir(ipfsDir)
	if err != nil {
		return err
	}
	fromVer, err := RepoVersion(ipfsDir)
	if err != nil {
		return fmt.Errorf("could not get repo version: %s", err)
	}
	if fromVer == targetVer {
		// repo already at target version number
		return nil
	}
	if fromVer > targetVer && !allowDowngrade {
		return fmt.Errorf("downgrade not allowed from %d to %d", fromVer, targetVer)
	}

	logger := log.New(os.Stdout, "", 0)

	logger.Print("Looking for suitable migration binaries.")

	migrations, binPaths, err := findMigrations(ctx, fromVer, targetVer)
	if err != nil {
		return err
	}

	// Download migrations that were not found
	if len(binPaths) < len(migrations) {
		missing := make([]string, 0, len(migrations)-len(binPaths))
		for _, mig := range migrations {
			if _, ok := binPaths[mig]; !ok {
				missing = append(missing, mig)
			}
		}

		logger.Println("Need", len(missing), "migrations, downloading.")

		tmpDir, err := ioutil.TempDir("", "migrations")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		fetched, err := fetchMigrations(ctx, fetcher, missing, tmpDir, logger)
		if err != nil {
			logger.Print("Failed to download migrations.")
			return err
		}

		for i := range missing {
			binPaths[missing[i]] = fetched[i]
		}
	}

	var revert bool
	if fromVer > targetVer {
		revert = true
	}
	for _, migration := range migrations {
		logger.Println("Running migration", migration, "...")
		err = runMigration(ctx, binPaths[migration], ipfsDir, revert, logger)
		if err != nil {
			return fmt.Errorf("migration %s failed: %s", migration, err)
		}
	}
	logger.Printf("Success: fs-repo migrated to version %d.\n", targetVer)

	return nil
}

func NeedMigration(target int) (bool, error) {
	vnum, err := RepoVersion("")
	if err != nil {
		return false, fmt.Errorf("could not get repo version: %s", err)
	}

	return vnum != target, nil
}

func ExeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// ReadMigrationConfig reads the Migration section of the IPFS config, avoiding
// reading anything other than the Migration section. That way, we're free to
// make arbitrary changes to all _other_ sections in migrations.
func ReadMigrationConfig(repoRoot string) (*config.Migration, error) {
	var cfg struct {
		Migration config.Migration
	}

	cfgPath, err := config.Filename(repoRoot)
	if err != nil {
		return nil, err
	}

	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		return nil, err
	}
	defer cfgFile.Close()

	err = json.NewDecoder(cfgFile).Decode(&cfg)
	if err != nil {
		return nil, err
	}

	switch cfg.Migration.Keep {
	case "":
		cfg.Migration.Keep = config.DefaultMigrationKeep
	case "discard", "cache", "keep":
	default:
		return nil, errors.New("unknown config value, Migrations.Keep must be 'cache', 'pin', or 'discard'")
	}

	if len(cfg.Migration.DownloadSources) == 0 {
		cfg.Migration.DownloadSources = config.DefaultMigrationDownloadSources
	}

	return &cfg.Migration, nil
}

// GetMigrationFetcher creates one or more fetchers according to
// downloadSources,
func GetMigrationFetcher(downloadSources []string, distPath string, newIpfsFetcher func(string) Fetcher) (Fetcher, error) {
	const httpUserAgent = "go-ipfs"

	var fetchers []Fetcher
	for _, src := range downloadSources {
		src := strings.TrimSpace(src)
		switch src {
		case "HTTPS", "https", "HTTP", "http":
			fetchers = append(fetchers, NewHttpFetcher(distPath, "", httpUserAgent, 0))
		case "IPFS", "ipfs":
			if newIpfsFetcher != nil {
				fetchers = append(fetchers, newIpfsFetcher(distPath))
			}
		default:
			u, err := url.Parse(src)
			if err != nil {
				return nil, fmt.Errorf("bad gateway address: %s", err)
			}
			switch u.Scheme {
			case "":
				u.Scheme = "https"
			case "https", "http":
			default:
				return nil, errors.New("bad gateway address: url scheme must be http or https")
			}
			fetchers = append(fetchers, NewHttpFetcher(distPath, u.String(), httpUserAgent, 0))
		case "":
			// Ignore empty string
		}
	}
	if len(fetchers) == 0 {
		return nil, errors.New("no sources specified")
	}

	if len(fetchers) == 1 {
		return fetchers[0], nil
	}

	// Wrap fetchers in a MultiFetcher to try them in order
	return NewMultiFetcher(fetchers...), nil
}

func migrationName(from, to int) string {
	return fmt.Sprintf("fs-repo-%d-to-%d", from, to)
}

// findMigrations returns a list of migrations, ordered from first to last
// migration to apply, and a map of locations of migration binaries of any
// migrations that were found.
func findMigrations(ctx context.Context, from, to int) ([]string, map[string]string, error) {
	step := 1
	count := to - from
	if from > to {
		step = -1
		count = from - to
	}

	migrations := make([]string, 0, count)
	binPaths := make(map[string]string, count)

	for cur := from; cur != to; cur += step {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		var migName string
		if step == -1 {
			migName = migrationName(cur+step, cur)
		} else {
			migName = migrationName(cur, cur+step)
		}
		migrations = append(migrations, migName)
		bin, err := exec.LookPath(migName)
		if err != nil {
			continue
		}
		binPaths[migName] = bin
	}
	return migrations, binPaths, nil
}

func runMigration(ctx context.Context, binPath, ipfsDir string, revert bool, logger *log.Logger) error {
	pathArg := fmt.Sprintf("-path=%s", ipfsDir)
	var cmd *exec.Cmd
	if revert {
		logger.Println("  => Running:", binPath, pathArg, "-verbose=true -revert")
		cmd = exec.CommandContext(ctx, binPath, pathArg, "-verbose=true", "-revert")
	} else {
		logger.Println("  => Running:", binPath, pathArg, "-verbose=true")
		cmd = exec.CommandContext(ctx, binPath, pathArg, "-verbose=true")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// fetchMigrations downloads the requested migrations, and returns a slice with
// the paths of each binary, in the same order specified by needed.
func fetchMigrations(ctx context.Context, fetcher Fetcher, needed []string, destDir string, logger *log.Logger) ([]string, error) {
	osv, err := osWithVariant()
	if err != nil {
		return nil, err
	}
	if osv == "linux-musl" {
		return nil, fmt.Errorf("linux-musl not supported, you must build the binary from source for your platform")
	}

	var wg sync.WaitGroup
	wg.Add(len(needed))
	bins := make([]string, len(needed))
	// Download and unpack all requested migrations concurrently.
	for i, name := range needed {
		logger.Printf("Downloading migration: %s...", name)
		go func(i int, name string) {
			defer wg.Done()
			dist := path.Join(distMigsRoot, name)
			ver, err := LatestDistVersion(ctx, fetcher, dist, false)
			if err != nil {
				logger.Printf("could not get latest version of migration %s: %s", name, err)
				return
			}
			loc, err := FetchBinary(ctx, fetcher, dist, ver, name, destDir)
			if err != nil {
				logger.Printf("could not download %s: %s", name, err)
				return
			}
			logger.Printf("Downloaded and unpacked migration: %s (%s)", loc, ver)
			bins[i] = loc
		}(i, name)
	}
	wg.Wait()

	var fails []string
	for i := range bins {
		if bins[i] == "" {
			fails = append(fails, needed[i])
		}
	}
	if len(fails) != 0 {
		err = fmt.Errorf("failed to download migrations: %s", strings.Join(fails, " "))
		if ctx.Err() != nil {
			err = fmt.Errorf("%s, %s", ctx.Err(), err)
		}
		return nil, err
	}

	return bins, nil
}
