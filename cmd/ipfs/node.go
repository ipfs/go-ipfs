package main

import (
	"context"
	"fmt"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	migrate "github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
)

type nodeBuilder struct {
	ctx      context.Context
	repoPath string

	migrate     bool
	userMigrate bool // --migrate flag present
	unencrypted bool

	routing string

	offline bool
	ipnsps  bool
	pubsub  bool
	mplex   bool

	permanent bool // only used for bloom filter in blockstore
	daemon    bool
}

func openRepo(path string, askMigrate, doMigrate bool) (repo.Repo, error) {
	repo, err := fsrepo.Open(path)
	switch err {
	default:
		return nil, err
	case fsrepo.ErrNeedMigration:
		fmt.Println("Found outdated fs-repo, migrations need to be run.")

		if askMigrate {
			doMigrate = YesNoPrompt("Run migrations now? [y/N]")
		}

		if !doMigrate {
			fmt.Println("Not running migrations of fs-repo now.")
			fmt.Println("Please get fs-repo-migrations from https://dist.ipfs.io")
			return nil, fmt.Errorf("fs-repo requires migration")
		}

		err = migrate.RunMigration(fsrepo.RepoVersion)
		if err != nil {
			fmt.Println("The migrations of fs-repo failed:")
			fmt.Printf("  %s\n", err)
			fmt.Println("If you think this is a bug, please file an issue and include this whole log output.")
			fmt.Println("  https://github.com/ipfs/fs-repo-migrations")
			return nil, err
		}

		repo, err = fsrepo.Open(path)
		if err != nil {
			return nil, err
		}
	case nil:
		break
	}
	return repo, nil
}

func (b *nodeBuilder) buildNode() (*core.IpfsNode, error) {
	if b.unencrypted {
		log.Errorf(`Running with --%s: All connections are UNENCRYPTED.
		You will not be able to connect to regular encrypted networks.`, unencryptTransportKwd)
	}

	repo, err := openRepo(b.repoPath, !b.userMigrate, b.migrate)
	if err != nil {
		return nil, err
	}

	cfg, err := repo.Config()
	if err != nil {
		repo.Close()
		return nil, err
	}

	routing := b.routing
	if routing == routingOptionDefaultKwd {
		routing = cfg.Routing.Type
		if routing == "" {
			routing = routingOptionDHTKwd
		}
	}

	var routingOption coreapi.Option
	switch routing {
	case "":
		routingOption = coreapi.Options() // keep default
	case routingOptionDHTKwd: // default
		routingOption = coreapi.Override(coreapi.Libp2pRouting, libp2p.DHTRouting(false))
	case routingOptionDHTClientKwd:
		routingOption = coreapi.Override(coreapi.Libp2pRouting, libp2p.DHTRouting(true))
	case routingOptionNoneKwd:
		routingOption = coreapi.Override(coreapi.Libp2pRouting, libp2p.NilRouting)
	default:
		repo.Close()
		return nil, fmt.Errorf("unrecognized routing option: %s", routing)
	}

	disablePubsub := coreapi.Options()
	ipnsPS := coreapi.Options()

	if !b.pubsub && !b.ipnsps {
		disablePubsub = coreapi.Disable(coreapi.Libp2pPubsub)
	}

	if b.ipnsps {
		ipnsPS = coreapi.Override(coreapi.Libp2pPubsubRouter, libp2p.PubsubRouter)
	}

	api, err := coreapi.New(
		coreapi.Ctx(b.ctx),

		coreapi.Online(b.daemon && !b.offline),
		coreapi.Repo(repo, coreapi.ParseConfig(), coreapi.Permanent(b.daemon)),
		coreapi.Override(coreapi.Libp2pSecurity, libp2p.Security(!b.unencrypted, cfg.Experimental.PreferTLS)),
		coreapi.Override(coreapi.Libp2pSmuxTransport, libp2p.SmuxTransport(b.mplex)),

		routingOption,
		disablePubsub,
		ipnsPS,
	)
	if err != nil {
		repo.Close()
		return nil, err
	}

	// nolint
	node := api.Node()
	node.IsDaemon = b.daemon
	return node, nil
}
