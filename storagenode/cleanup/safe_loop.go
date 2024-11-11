// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package cleanup

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"
)

var mon = monkit.Package()

// Availability checks if we have free resources to run background jobs.
type Availability interface {
	Available() (bool, error)
}

// SafeLoop is an execution.
type SafeLoop struct {
	log          *zap.Logger
	availability Availability

	lastFinished time.Time

	running bool
	cancel  context.CancelFunc

	config SafeLoopConfig
}

// SafeLoopConfig is the configuration for SafeLoop.
type SafeLoopConfig struct {
	CheckingPeriod time.Duration `help:"time period to check the availability condition" default:"3m"`
	RunPeriod      time.Duration `help:"minimum time between the execution of cleanup" default:"1h"`
}

// NewSafeLoop creates a new SafeLoop.
func NewSafeLoop(log *zap.Logger, availability Availability, cfg SafeLoopConfig) *SafeLoop {
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
	result := make(chan error)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(s.config.CheckingPeriod):
			var available bool
			if s.availability != nil {
				available, err = s.availability.Available()
				if err != nil {
					s.log.Warn("availabilityCondition is failed", zap.Error(err))
				}
			}
			if available && !s.running && time.Since(s.lastFinished) > s.config.RunPeriod {
				// Run the process
				ctx, s.cancel = context.WithCancel(ctx)
				s.running = true
				go func() {
					err := do(ctx)
					result <- err
				}()
			}
			if !available && s.running {
				s.log.Info("stopping the running chores, as load is too high")
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
