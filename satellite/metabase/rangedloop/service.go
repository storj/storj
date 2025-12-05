// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"fmt"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/sync2"
	"storj.io/eventkit"
)

var (
	mon = monkit.Package()
	ev  = eventkit.Package()

	// Error is a standard error class for this component.
	Error = errs.Class("ranged loop")
)

// Config contains configurable values for the shared loop.
type Config struct {
	Parallelism          int           `help:"how many chunks of segments to process in parallel" default:"2"`
	BatchSize            int           `help:"how many items to query in a batch" default:"2500"`
	AsOfSystemInterval   time.Duration `help:"as of system interval" releaseDefault:"-5m" devDefault:"-1us" testDefault:"-1us"`
	Interval             time.Duration `help:"how often to run the loop" releaseDefault:"2h" devDefault:"10s" testDefault:"0"`
	SpannerStaleInterval time.Duration `help:"sets spanner stale read timestamp as now()-interval" default:"0"`
	// TODO: remove this flag when we will know which type is optimal for ranged loop
	TestingSpannerQueryType string `help:"use to select query type which will be used to execute ranged loop (sql|read)" default:"" testDefault:"read" hidden:"true"`

	SuspiciousProcessedRatio float64 `help:"ratio where to consider processed count as supicious" default:"0.03"`
}

// Service iterates through all segments and calls the attached observers for every segment
//
// architecture: Service
type Service struct {
	log       *zap.Logger
	config    Config
	provider  RangeSplitter
	observers []Observer

	Loop *sync2.Cycle
}

// NewService creates a new instance of the ranged loop service.
func NewService(log *zap.Logger, config Config, provider RangeSplitter, observers []Observer) *Service {
	return &Service{
		log:       log,
		config:    config,
		provider:  provider,
		observers: observers,
		Loop:      sync2.NewCycle(config.Interval),
	}
}

// observerState contains information to manage an observer during a loop iteration.
type observerState struct {
	observer       Observer
	rangeObservers []*rangeObserverState
	// err is the error that occurred during the observer's Start method.
	// If err is set, the observer will be skipped during the loop iteration.
	err error
}

type rangeObserverState struct {
	rangeObserver Partial
	duration      time.Duration
	// err is the error that is returned by the observer's Fork or Process method.
	// If err is set, the range observer will be skipped during the loop iteration.
	err error
}

// ObserverDuration reports back on how long it took the observer to process all the segments.
type ObserverDuration struct {
	Observer Observer
	// Duration is set to -1 when the observer has errored out
	// so someone watching metrics can tell that something went wrong.
	Duration time.Duration
}

// Close stops the ranged loop.
func (service *Service) Close() error {
	service.Loop.Close()
	return nil
}

// Run starts the looping service.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if service.config.Interval == 0 {
		return nil
	}

	service.log.Info("ranged loop initialized")

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		_, err := service.RunOnce(ctx)
		if err != nil {
			if errs2.IsCanceled(err) {
				return err
			}

			if ctx.Err() != nil {
				return errs.Combine(err, ctx.Err())
			}
		}

		return nil
	})
}

// RunOnce goes through one time and sends information to observers.
func (service *Service) RunOnce(ctx context.Context) (observerDurations []ObserverDuration, err error) {
	defer mon.Task()(&ctx)(&err)

	service.log.Info("ranged loop started",
		zap.Int("parallelism", service.config.Parallelism),
		zap.Int("batch_size", service.config.BatchSize),
		zap.Duration("asofsystem_interval", service.config.AsOfSystemInterval),
		zap.Duration("spannerstale_interval", service.config.SpannerStaleInterval),
	)

	defer func() {
		if err != nil {
			service.log.Error("ranged loop failure", zap.Error(err))

			mon.Event("rangedloop_error")
		} else {
			service.log.Info("ranged loop finished")
		}
	}()

	observerStates, err := startObservers(ctx, service.log, service.observers)
	if err != nil {
		return nil, err
	}

	rangeProviders, err := service.provider.CreateRanges(ctx, service.config.Parallelism, service.config.BatchSize)
	if err != nil {
		return nil, err
	}

	group := errs2.Group{}
	for index, rangeProvider := range rangeProviders {
		uuidRange := rangeProvider.Range()
		service.log.Debug("creating range", zap.Int("index", index), zap.Stringer("start", uuidRange.Start), zap.Stringer("end", uuidRange.End))

		rangeObservers := []*rangeObserverState{}
		for i, observerState := range observerStates {
			if observerState.err != nil {
				service.log.Debug("observer returned error", zap.Error(observerState.err))
				continue
			}
			rangeObserver, err := observerState.observer.Fork(ctx)
			rangeState := &rangeObserverState{
				rangeObserver: rangeObserver,
				err:           err,
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

	return finishObservers(ctx, service.log, observerStates), nil
}

func createGoroutineClosure(ctx context.Context, rangeProvider SegmentProvider, states []*rangeObserverState) func() error {
	return func() (err error) {
		defer mon.Task()(&ctx)(&err)

		return rangeProvider.Iterate(ctx, func(segments []Segment) error {
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

func startObservers(ctx context.Context, log *zap.Logger, observers []Observer) (observerStates []observerState, err error) {
	startTime := time.Now()

	for _, obs := range observers {
		observerStates = append(observerStates, startObserver(ctx, log, startTime, obs))
	}

	return observerStates, nil
}

func startObserver(ctx context.Context, log *zap.Logger, startTime time.Time, observer Observer) observerState {
	err := observer.Start(ctx, startTime)

	if err != nil {
		log.Error(
			"Starting observer failed. This observer will be excluded from this run of the ranged segment loop.",
			zap.String("observer", fmt.Sprintf("%T", observer)),
			zap.Error(err),
		)
	}

	return observerState{
		observer: observer,
		err:      err,
	}
}

func finishObservers(ctx context.Context, log *zap.Logger, observerStates []observerState) (observerDurations []ObserverDuration) {
	for _, state := range observerStates {
		observerDurations = append(observerDurations, finishObserver(ctx, log, state))
	}

	sendObserverDurations(observerDurations)

	return observerDurations
}

// Iterating over the segments is done.
// This is the reduce step.
func finishObserver(ctx context.Context, log *zap.Logger, state observerState) ObserverDuration {
	if state.err != nil {
		return ObserverDuration{
			Observer: state.observer,
			Duration: -1 * time.Second,
		}
	}
	for _, rangeObserver := range state.rangeObservers {
		if rangeObserver.err != nil {
			log.Error(
				"Observer failed during Process(), it will not be finalized in this run of the ranged segment loop",
				zap.String("observer", fmt.Sprintf("%T", state.observer)),
				zap.Error(rangeObserver.err),
			)
			return ObserverDuration{
				Observer: state.observer,
				Duration: -1 * time.Second,
			}
		}
	}

	var duration time.Duration
	for _, rangeObserver := range state.rangeObservers {
		err := state.observer.Join(ctx, rangeObserver.rangeObserver)
		if err != nil {
			log.Error(
				"Observer failed during Join(), it will not be finalized in this run of the ranged segment loop",
				zap.String("observer", fmt.Sprintf("%T", state.observer)),
				zap.Error(rangeObserver.err),
			)
			return ObserverDuration{
				Observer: state.observer,
				Duration: -1 * time.Second,
			}
		}
		duration += rangeObserver.duration
	}

	err := state.observer.Finish(ctx)
	if err != nil {
		log.Error(
			"Observer failed during Finish()",
			zap.String("observer", fmt.Sprintf("%T", state.observer)),
			zap.Error(err),
		)
		return ObserverDuration{
			Observer: state.observer,
			Duration: -1 * time.Second,
		}
	}

	return ObserverDuration{
		Duration: duration,
		Observer: state.observer,
	}
}

func processBatch(ctx context.Context, states []*rangeObserverState, segments []Segment) (err error) {
	for _, state := range states {
		if state.err != nil {
			// this observer has errored in a previous batch
			continue
		}
		start := time.Now()
		err := state.rangeObserver.Process(ctx, segments)
		state.duration += time.Since(start)
		if err != nil {
			// unsure if this is necessary here
			if errs2.IsCanceled(err) {
				return err
			}
			state.err = err
		}
	}
	return nil
}
