// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"
)

var mon = monkit.Package()

// Enablement checks if we have free resources to run background jobs.
type Enablement interface {
	Enabled() (bool, error)
}

// SafeLoop is an execution.
type SafeLoop struct {
	log          *zap.Logger
	availability []Enablement

	lastFinished time.Time

	running bool
	cancel  context.CancelFunc

	config SafeLoopConfig
}

// SafeLoopConfig is the configuration for SafeLoop.
type SafeLoopConfig struct {
	CheckingPeriod time.Duration `help:"time period to check the availability condition" default:"1m"`
	RunPeriod      time.Duration `help:"minimum time between the execution of cleanup" default:"15m"`
}

// NewSafeLoop creates a new SafeLoop.
func NewSafeLoop(log *zap.Logger, availability []Enablement, cfg SafeLoopConfig) *SafeLoop {
	return &SafeLoop{
		availability: availability,
		config:       cfg,
		log:          log,
	}
}

// RunSafe runs the given function in a loop until it returns an error or the context is done.
// If stops the function, with context cancellation, if condition is false (for example: if load is high).
func (s *SafeLoop) RunSafe(ctx context.Context, do func(ctx context.Context) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	result := make(chan error, 1)
	// initial run after 10 seconds
	period := 0 * time.Second
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(period):
			period = s.config.CheckingPeriod
			enable := true
			for _, check := range s.availability {
				vote, err := check.Enabled()
				if err != nil {
					s.log.Warn("availabilityCondition is failed", zap.Error(err))
				}
				if !vote {
					s.log.Debug("cleanup chore is not running as availability condition is not met", zap.String("condition", fmt.Sprintf("%T", check)))
				}
				enable = enable && vote
			}
			if enable && !s.running && time.Since(s.lastFinished) > s.config.RunPeriod {
				s.log.Info("starting cleanup chores")
				// Run the process
				var processCtx context.Context
				processCtx, s.cancel = context.WithCancel(ctx)
				s.running = true
				go func() {
					err := do(processCtx)
					result <- err
				}()
			}
			if !enable && s.running {
				s.log.Info("stopping the running chores, as running conditions are not met")
				s.lastFinished = time.Now()
				// stop the process
				s.cancel()
			}
		case err := <-result:
			s.running = false
			if err != nil {
				s.log.Warn("execution is failed", zap.Error(err))
			}
			s.lastFinished = time.Now()
		}
	}
}
