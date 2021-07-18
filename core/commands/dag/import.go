package dagcmd

import (
	"context"
	"fmt"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/interface-go-ipfs-core/options"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

type dagImportAPI interface {
	ImportMany(ctx context.Context, directory files.Directory, pinRoots bool) (<-chan coreapi.RootMeta, <-chan error)
}

func dagImport(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}

	// on import ensure we do not reach out to the network for any reason
	// if a pin based on what is imported + what is in the blockstore
	// isn't possible: tough luck
	api, err = api.WithOptions(options.Api.Offline(true))
	if err != nil {
		return err
	}

	dagapi, ok := api.Dag().(dagImportAPI)
	if !ok {
		return fmt.Errorf("API does not support DAG import")
	}

	doPinRoots, _ := req.Options[pinRootsOptionName].(bool)

	resCh, errCh := dagapi.ImportMany(req.Context, req.Files, doPinRoots)

	err = <-errCh
	if err != nil {
		return err
	}

	if !doPinRoots {
		return nil
	}

	var failedPins, totalRoots int

	for ret := range resCh {
		if ret.PinErrorMsg != "" {
			failedPins++
		}
		totalRoots++

		if err := res.Emit(&CarImportOutput{Root: ret}); err != nil {
			return err
		}
	}

	if doPinRoots && failedPins > 0 {
		return fmt.Errorf(
			"unable to pin all roots: %d out of %d failed",
			failedPins,
			totalRoots,
		)
	}

	return nil
}
