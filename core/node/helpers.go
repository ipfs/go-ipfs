package node

import (
	"context"

	"github.com/jbenet/goprocess"
	"github.com/pkg/errors"
	"go.uber.org/fx"
)

type lcProcess struct {
	fx.In

	LC   fx.Lifecycle
	Proc goprocess.Process
}

// Append wraps ProcessFunc into a goprocess, and appends it to the lifecycle
func (lp *lcProcess) Append(f goprocess.ProcessFunc) {
	// Hooks are guaranteed to run in sequence. If a hook fails to start, its
	// OnStop won't be executed.
	var proc goprocess.Process

	lp.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			proc = lp.Proc.Go(f)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if proc == nil { // Theoretically this shouldn't ever happen
				return errors.New("lcProcess: proc was nil")
			}

			return proc.Close() // todo: respect ctx, somehow
		},
	})
}
