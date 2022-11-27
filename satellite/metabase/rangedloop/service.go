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
	Parallelism        int           `help:"how many chunks of segments to process in parallel" default:"2"`
	BatchSize          int           `help:"how many items to query in a batch" default:"2500"`
	AsOfSystemInterval time.Duration `help:"as of system interval" releaseDefault:"-5m" devDefault:"-1us" testDefault:"-1us"`
}

// Service iterates through all segments and calls the attached observers for every segment
//
// architecture: Service
type Service struct {
	log       *zap.Logger
	config    Config
	provider  RangeSplitter
	observers []Observer
}

// NewService creates a new instance of the ranged loop service.
func NewService(log *zap.Logger, config Config, provider RangeSplitter, observers []Observer) *Service {
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
	rangeObservers []*rangeObserverState
}

type rangeObserverState struct {
	rangeObserver Partial
	duration      time.Duration
}

// ObserverDuration reports back on how long it took the observer to process all the segments.
type ObserverDuration struct {
	Observer Observer
	Duration time.Duration
}

// Run starts the looping service.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		_, err := service.RunOnce(ctx)
		if err != nil {
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
func (service *Service) RunOnce(ctx context.Context) (observerDurations []ObserverDuration, err error) {
	defer mon.Task()(&ctx)(&err)

	observerStates, err := startObservers(ctx, service.observers)
	if err != nil {
		return nil, err
	}

	rangeProviders, err := service.provider.CreateRanges(service.config.Parallelism, service.config.BatchSize)
	if err != nil {
		return nil, err
	}

	group := errs2.Group{}
	for _, rangeProvider := range rangeProviders {
		rangeObservers := []*rangeObserverState{}
		for i, observerState := range observerStates {
			rangeObserver, err := observerState.observer.Fork(ctx)
			if err != nil {
				return nil, err
			}
			rangeState := &rangeObserverState{
				rangeObserver: rangeObserver,
			}
			rangeObservers = append(rangeObservers, rangeState)
			observerStates[i].rangeObservers = append(observerStates[i].rangeObservers, rangeState)
		}

		// Create closure to capture loop variables.
		group.Go(createGoroutineClosure(ctx, rangeProvider, rangeObservers))
	}

	// Improvement: stop all ranges when one has an error.
	errList := group.Wait()
	if errList != nil {
		return nil, errs.Combine(errList...)
	}

	return finishObservers(ctx, observerStates)
}

func createGoroutineClosure(ctx context.Context, rangeProvider SegmentProvider, states []*rangeObserverState) func() error {
	return func() (err error) {
		defer mon.Task()(&ctx)(&err)

		return rangeProvider.Iterate(ctx, func(segments []segmentloop.Segment) error {
			// check for cancellation every segment batch
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return processBatch(ctx, states, segments)
			}
		})
	}
}

func startObservers(ctx context.Context, observers []Observer) (observerStates []observerState, err error) {
	startTime := time.Now()

	for _, obs := range observers {
		state, err := startObserver(ctx, startTime, obs)
		if err != nil {
			return nil, err
		}

		observerStates = append(observerStates, state)
	}

	return observerStates, nil
}

func startObserver(ctx context.Context, startTime time.Time, observer Observer) (observerState, error) {
	err := observer.Start(ctx, startTime)

	return observerState{
		observer: observer,
	}, err
}

func finishObservers(ctx context.Context, observerStates []observerState) (observerDurations []ObserverDuration, err error) {
	for _, state := range observerStates {
		observerDuration, err := finishObserver(ctx, state)
		if err != nil {
			return nil, err
		}

		observerDurations = append(observerDurations, observerDuration)
	}

	sendObserverDurations(observerDurations)

	return observerDurations, nil
}

// Iterating over the segments is done.
// This is the reduce step.
func finishObserver(ctx context.Context, state observerState) (ObserverDuration, error) {
	var duration time.Duration
	for _, rangeObserver := range state.rangeObservers {
		err := state.observer.Join(ctx, rangeObserver.rangeObserver)
		if err != nil {
			return ObserverDuration{}, err
		}
		duration += rangeObserver.duration
	}

	return ObserverDuration{
		Duration: duration,
		Observer: state.observer,
	}, state.observer.Finish(ctx)
}

func processBatch(ctx context.Context, states []*rangeObserverState, segments []segmentloop.Segment) (err error) {
	for _, state := range states {
		start := time.Now()
		err := state.rangeObserver.Process(ctx, segments)
		if err != nil {
			return err
		}
		state.duration += time.Since(start)
	}
	return nil
}
