package corerepo

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/ipfs/go-ipfs/core"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"

	humanize "github.com/dustin/go-humanize"
	dsq "github.com/ipfs/go-datastore/query"
)

// SizeStat wraps information about the repository size and its limit.
type SizeStat struct {
	RepoSize   uint64 // size in bytes
	StorageMax uint64 // size in bytes
}

// Stat wraps information about the objects stored on disk.
type Stat struct {
	SizeStat

	// Number of blocks. Called "objects" for historical reasons".
	NumObjects uint64

	// Number of provider records.
	NumProviders uint64

	// Total number of values in the datastore.
	NumValues uint64

	RepoPath string
	Version  string
}

// NoLimit represents the value for unlimited storage
const NoLimit uint64 = math.MaxUint64

// RepoStat returns a *Stat object with all the fields set.
func RepoStat(ctx context.Context, n *core.IpfsNode) (stat Stat, err error) {
	stat.Version = fmt.Sprintf("fs-repo@%d", fsrepo.RepoVersion)
	stat.RepoPath, err = fsrepo.BestKnownPath()
	if err != nil {
		return stat, err
	}
	stat.SizeStat, err = RepoSize(ctx, n)
	if err != nil {
		return stat, err
	}

	// count the records.
	query, err := n.Repo.Datastore().Query(dsq.Query{KeysOnly: true})
	if err != nil {
		return stat, err
	}
	defer query.Close()

	for {
		if err := ctx.Err(); err != nil {
			return stat, err
		}
		res, ok := query.NextSync()
		if !ok {
			return stat, nil
		}
		if res.Error != nil {
			return stat, res.Error
		}
		stat.NumValues++
		if strings.HasPrefix(res.Key, "/blocks/") {
			stat.NumObjects++
		} else if strings.HasPrefix(res.Key, "/providers/") {
			stat.NumProviders++
		}
	}
}

// RepoSize returns a *Stat object with the RepoSize and StorageMax fields set.
func RepoSize(ctx context.Context, n *core.IpfsNode) (SizeStat, error) {
	r := n.Repo

	cfg, err := r.Config()
	if err != nil {
		return SizeStat{}, err
	}

	usage, err := r.GetStorageUsage()
	if err != nil {
		return SizeStat{}, err
	}

	storageMax := NoLimit
	if cfg.Datastore.StorageMax != "" {
		storageMax, err = humanize.ParseBytes(cfg.Datastore.StorageMax)
		if err != nil {
			return SizeStat{}, err
		}
	}

	return SizeStat{
		RepoSize:   usage,
		StorageMax: storageMax,
	}, nil
}
