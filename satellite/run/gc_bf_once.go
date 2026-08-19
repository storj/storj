// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/debug"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/mud"
)

// GcBfOnce is a subcommand to start only a ranged loop with BF generation.
type GcBfOnce struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *GcBfOnce) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	mud.Provide[*OnceRunner](ball, NewOnceRunner)
	return mud.Or(
		mud.Select[debug.Wrapper](ball),
		mud.Select[*DBVersionCheck](ball),
		mud.Select[*OnceRunner](ball))
}

// OnceRunner is a wrapper to run the ranged loop once and then stop the
// application.
type OnceRunner struct {
	log       *zap.Logger
	config    rangedloop.Config
	db        *metabase.DB
	provider  rangedloop.RangeSplitter
	observers []rangedloop.Observer
	trigger   *modular.StopTrigger
}

// NewOnceRunner creates a new OnceRunner.
func NewOnceRunner(log *zap.Logger, config rangedloop.Config, db *metabase.DB, provider rangedloop.RangeSplitter, observers []rangedloop.Observer, trigger *modular.StopTrigger) *OnceRunner {
	return &OnceRunner{
		log:       log,
		config:    config,
		db:        db,
		provider:  provider,
		observers: observers,
		trigger:   trigger,
	}
}

// Run pins the snapshot to read, runs the ranged loop once and then stops the
// application.
func (o *OnceRunner) Run(ctx context.Context) (err error) {
	defer o.trigger.Cancel()

	safepoint, err := rangedloop.HoldSafepoint(ctx, o.log, o.db, o.config.Safepoint)
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, safepoint.Close())
	}()

	// scan at the pinned timestamp; abort the run if any hold is ever lost
	ctx, cancel := safepoint.Context(ctx)
	defer cancel()

	// One read timestamp for the whole job, so that everything the run
	// publishes describes the same snapshot.
	readTimestamp := safepoint.ReadTime()
	if readTimestamp.IsZero() && o.config.StaleInterval > 0 {
		readTimestamp = time.Now().Add(-o.config.StaleInterval)
	}
	provider, observers := o.provider, o.observers
	if _, metabaseSource := provider.(*rangedloop.MetabaseRangeSplitter); !metabaseSource {
		// The splitter is a replaceable component, so the scan may read
		// something else entirely (the Avro export). Those sources are one
		// snapshot by construction and carry no read timestamp, so rebuilding
		// them off one would silently scan the metabase instead.
		if !safepoint.ReadTime().IsZero() {
			return errs.New("a safepoint pins a metabase snapshot, but the scan reads %T", provider)
		}
	} else if !readTimestamp.IsZero() {
		provider = rangedloop.NewMetabaseRangeSplitterWithReadTimestamp(o.log.Named("rangedloop-metabase-range-splitter"), o.db, o.config, readTimestamp)
		observers = rangedloop.AddSegmentsCountChecks(o.log.Named("rangedloop"), o.db, readTimestamp, observers)
	}

	// its own service, rather than the component: the component has a Run
	// method, which the modular runner would start as a second, periodic loop
	// over the same observers
	service := rangedloop.NewService(o.log, o.config, provider, observers)
	defer func() {
		err = errs.Combine(err, service.Close())
	}()

	durations, err := service.RunOnce(ctx)
	if err != nil {
		return err
	}
	// The ranged loop only logs observer failures, so the job would otherwise
	// report success after producing nothing. It does not hold anything back:
	// an observer that fails after the bloom filter observer has published
	// cannot take that generation away again, and only the publish guard set
	// above refuses to publish one in the first place.
	return rangedloop.ObserverError(durations)
}
