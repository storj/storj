// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/satellite/metainfo"
)

var (
	// Error defines the metrics chore errors class.
	Error = errs.Class("metrics chore error")
	mon   = monkit.Package()
)

// Config contains configurable values for metrics collection.
type Config struct {
	// TODO: confirm reasonable interval.
	Interval time.Duration `help:"the time between each metrics run" releaseDefault:"120h" devDefault:"10m"`
}

// Chore implements the metrics chore.
//
// architecture: Chore
type Chore struct {
	log          *zap.Logger
	config       Config
	Loop         sync2.Cycle
	metainfoLoop *metainfo.Loop
}

// NewChore creates a new instance of the metrics chore.
func NewChore(log *zap.Logger, config Config, loop *metainfo.Loop) *Chore {
	return &Chore{
		log:          log,
		config:       config,
		Loop:         *sync2.NewCycle(config.Interval),
		metainfoLoop: loop,
	}
}

// Run starts the metrics service.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		counter := newCounter()

		err = chore.metainfoLoop.Join(ctx, counter)
		if err != nil {
			chore.log.Error("error joining metainfoloop", zap.Error(err))
			return nil
		}
		mon.IntVal("remote_dependent_object_count").Observe(counter.remoteDependentObjectCount)
		mon.IntVal("inline_object_count").Observe(counter.inlineObjectCount)
		mon.IntVal("total_object_count").Observe(counter.totalObjectCount)

		return nil
	})
}

// Close closes metrics chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
