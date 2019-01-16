// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
)

// Config contains configurable values for tally
type Config struct {
	Interval time.Duration `help:"how frequently tally should run" default:"30s"`
}

// Initialize a tally struct
func (c Config) initialize(ctx context.Context) (Tally, error) {
	pointerdb := pointerdb.LoadFromContext(ctx)
	if pointerdb == nil {
		return nil, Error.New("programmer error: pointerdb responsibility unstarted")
	}
	overlay := overlay.LoadServerFromContext(ctx)
	if overlay == nil {
		return nil, Error.New("programmer error: overlay responsibility unstarted")
	}
	db, ok := ctx.Value("masterdb").(interface {
		BandwidthAgreement() bwagreement.DB
		Accounting() accounting.DB
	})
	if !ok {
		return nil, Error.Wrap(errs.New("unable to get master db instance"))
	}
	return newTally(zap.L(), db.Accounting(), db.BandwidthAgreement(), pointerdb, overlay, 0, c.Interval), nil
}

// Run runs the tally with configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	tally, err := c.initialize(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		if err := tally.Run(ctx); err != nil {
			defer cancel()
			zap.L().Debug("Tally is shutting down", zap.Error(err))
		}
	}()

	return server.Run(ctx)
}
