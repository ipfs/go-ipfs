package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations/ipfsfetcher"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
)

// addMigrations adds any migration downloaded by the fetcher to the IPFS node
func addMigrations(ctx context.Context, node *core.IpfsNode, fetcher migrations.Fetcher, pin bool) error {
	var fetchers []migrations.Fetcher
	if mf, ok := fetcher.(*migrations.MultiFetcher); ok {
		fetchers = mf.Fetchers()
	} else {
		fetchers = []migrations.Fetcher{fetcher}
	}

	for _, fetcher := range fetchers {
		switch f := fetcher.(type) {
		case *ipfsfetcher.IpfsFetcher:
			// Add migrations by connecting to temp node and getting from IPFS
			err := addMigrationPaths(ctx, node, f.AddrInfo(), f.FetchedPaths(), pin)
			if err != nil {
				return err
			}
		case *migrations.HttpFetcher:
			// Add the downloaded migration files directly
			if migrations.DownloadDirectory != "" {
				var paths []string
				err := filepath.Walk(migrations.DownloadDirectory, func(filePath string, info os.FileInfo, err error) error {
					if info.IsDir() {
						return nil
					}
					paths = append(paths, filePath)
					return nil
				})
				if err != nil {
					return err
				}
				err = addMigrationFiles(ctx, node, paths, pin)
				if err != nil {
					return err
				}
			}
		default:
			return errors.New("Cannot get migrations from unknown fetcher type")
		}
	}

	return nil
}

// addMigrationFiles adds the files at paths to IPFS, optionally pinning them
func addMigrationFiles(ctx context.Context, node *core.IpfsNode, paths []string, pin bool) error {
	if len(paths) == 0 {
		return nil
	}
	ifaceCore, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return err
	}
	ufs := ifaceCore.Unixfs()

	// Add migration files
	for _, filePath := range paths {
		f, err := os.Open(filePath)
		if err != nil {
			return err
		}

		fi, err := f.Stat()
		if err != nil {
			return err
		}

		ipfsPath, err := ufs.Add(ctx, files.NewReaderStatFile(f, fi), options.Unixfs.Pin(pin))
		if err != nil {
			return err
		}
		fmt.Printf("Added migration file %q: %s\n", filepath.Base(filePath), ipfsPath)
	}

	return nil
}

// addMigrationPaths adds the files at paths to IPFS, optionally pinning
// them. This is done after connecting to the peer.
func addMigrationPaths(ctx context.Context, node *core.IpfsNode, peerInfo peer.AddrInfo, paths []ipath.Path, pin bool) error {
	if len(paths) == 0 {
		return errors.New("nothing downloaded by ipfs fetcher")
	}
	if len(peerInfo.Addrs) == 0 {
		return errors.New("no local swarm address for migration node")
	}

	ipfs, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return err
	}

	// Connect to temp node
	if err := ipfs.Swarm().Connect(ctx, peerInfo); err != nil {
		return fmt.Errorf("could not connect to migration peer %q: %s", peerInfo.ID, err)
	}
	fmt.Printf("connected to migration peer %q\n", peerInfo)

	if pin {
		pinApi := ipfs.Pin()
		for _, ipfsPath := range paths {
			err := pinApi.Add(ctx, ipfsPath)
			if err != nil {
				return err
			}
			fmt.Printf("Added and pinned migration file: %q\n", ipfsPath)
		}
		return nil
	}

	ufs := ipfs.Unixfs()

	// Add migration files
	for _, ipfsPath := range paths {
		err = ipfsGet(ctx, ufs, ipfsPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func ipfsGet(ctx context.Context, ufs coreiface.UnixfsAPI, ipfsPath ipath.Path) error {
	nd, err := ufs.Get(ctx, ipfsPath)
	if err != nil {
		return err
	}
	defer nd.Close()

	fnd, ok := nd.(files.File)
	if !ok {
		return fmt.Errorf("not a file node: %q", ipfsPath)
	}
	_, err = io.Copy(ioutil.Discard, fnd)
	if err != nil {
		return fmt.Errorf("cannot read migration: %w", err)
	}
	fmt.Printf("Added migration file: %q\n", ipfsPath)
	return nil
}
