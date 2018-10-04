// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/provider"
)

var (
	mon = monkit.Package()
	// Error is a standard error class for this package.
	Error = errs.Class("checker error")
)

// Config contains configurable values for checker
type Config struct {
	Interval time.Duration `help:"how frequently checker should audit segments" default:"30s"`
}

// Run runs the checker with configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	zap.S().Info("Checker is starting up")

	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		for {
			select {
			case <-ticker.C:
				zap.S().Info("Starting segment checker service")
			case <-ctx.Done():
				return
			}
		}
	}()

	return server.Run(ctx)
}
