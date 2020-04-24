// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package expireddeletion

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/metainfo"
)

var (
	// Error defines the expireddeletion chore errors class
	Error = errs.Class("expireddeletion chore error")
	mon   = monkit.Package()
)

// Config contains configurable values for expired segment cleanup
type Config struct {
	Interval time.Duration `help:"the time between each attempt to go through the db and clean up expired segments" releaseDefault:"120h" devDefault:"10m"`
	Enabled  bool          `help:"set if expired segment cleanup is enabled or not" releaseDefault:"true" devDefault:"true"`
}

// Chore implements the expired segment cleanup chore
//
// architecture: Chore
type Chore struct {
	log    *zap.Logger
	config Config
	Loop   *sync2.Cycle

	metainfo     *metainfo.Service
	metainfoLoop *metainfo.Loop
}

// NewChore creates a new instance of the expireddeletion chore
func NewChore(log *zap.Logger, config Config, meta *metainfo.Service, loop *metainfo.Loop) *Chore {
	return &Chore{
		log:          log,
		config:       config,
		Loop:         sync2.NewCycle(config.Interval),
		metainfo:     meta,
		metainfoLoop: loop,
	}
}

// Run starts the expireddeletion loop service
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !chore.config.Enabled {
		return nil
	}

	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		deleter := &expiredDeleter{
			log:      chore.log.Named("expired deleter observer"),
			metainfo: chore.metainfo,
		}

		// delete expired segments
		err = chore.metainfoLoop.Join(ctx, deleter)
		if err != nil {
			chore.log.Error("error joining metainfoloop", zap.Error(err))
			return nil
		}
		return nil
	})
}
