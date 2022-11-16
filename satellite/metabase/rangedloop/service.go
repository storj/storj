// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/storj/satellite/metabase/segmentloop"
)

var (
	mon = monkit.Package()
)

// Config contains configurable values for the shared loop.
type Config struct {
	Parellelism        int           `help:"how many chunks of segments to process in parallel" default:"1"`
	BatchSize          int           `help:"how many items to query in a batch" default:"2500"`
	AsOfSystemInterval time.Duration `help:"as of system interval" releaseDefault:"-5m" devDefault:"-1us" testDefault:"-1us"`
}

// Service iterates through all segments and calls the attached observers for every segment
//
// architecture: Service
type Service struct {
	log       *zap.Logger
	config    Config
	provider  Provider
	observers []Observer
}

// NewService creates a new instance of the ranged loop service.
func NewService(log *zap.Logger, config Config, provider Provider, observers []Observer) *Service {
	return &Service{
		log:       log,
		config:    config,
		provider:  provider,
		observers: observers,
	}
}

// observerState contains information to manage an observer during a loop iteration.
// Improvement: track duration.
type observerState struct {
	observer       Observer
	rangeObservers []Partial
}

// Run starts the looping service.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		if err := service.RunOnce(ctx); err != nil {
			service.log.Error("ranged loop failure", zap.Error(err))

			if errs2.IsCanceled(err) {
				return err
			}
			if ctx.Err() != nil {
				return errs.Combine(err, ctx.Err())
			}

			mon.Event("rangedloop_error") //mon:locked
		}
	}
}

// RunOnce goes through one time and sends information to observers.
func (service *Service) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	startTime := time.Now()
	observerStates := []observerState{}

	for _, obs := range service.observers {
		err := obs.Start(ctx, startTime)
		if err != nil {
			return err
		}

		observerStates = append(observerStates, observerState{
			observer: obs,
		})
	}

	rangeProviders, err := service.provider.CreateRanges(service.config.Parellelism, service.config.BatchSize)
	if err != nil {
		return err
	}

	group := errs2.Group{}
	for rangeIndex, rangeProvider := range rangeProviders {
		rangeObservers := []Partial{}
		for i, observerState := range observerStates {
			rangeObserver, err2 := observerState.observer.Fork(ctx)
			if err2 != nil {
				return err2
			}
			rangeObservers = append(rangeObservers, rangeObserver)
			observerStates[i].rangeObservers = append(observerStates[i].rangeObservers, rangeObserver)
		}

		// Create closure to capture loop variables.
		createClosure := func(rangeIndex int, rangeProvider RangeProvider, rangeObservers []Partial) func() error {
			return func() (err error) {
				defer mon.Task()(&ctx)(&err)

				return rangeProvider.Iterate(ctx, func(segments []segmentloop.Segment) error {
					for _, rangeObserver := range rangeObservers {
						err := rangeObserver.Process(ctx, segments)
						if err != nil {
							return err
						}
					}
					return nil
				})
			}
		}

		group.Go(createClosure(rangeIndex, rangeProvider, rangeObservers))
	}

	// Improvement: stop all ranges when one has an error.
	errList := group.Wait()
	if errList != nil {
		return errs.Combine(errList...)
	}

	// Segment loop has ended.
	// This is the reduce step.
	for _, state := range observerStates {
		for _, rangeObserver := range state.rangeObservers {
			err := state.observer.Join(ctx, rangeObserver)
			if err != nil {
				return err
			}
		}

		err := state.observer.Finish(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}
