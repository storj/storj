// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"

	"gopkg.in/spacemonkeygo/monkit.v2"

	"log"
)

var (
	mon = monkit.Package()
)

// Config contains configurable values for repairer
type Config struct {
}

// Run runs the repairer with configured values
func (c *Config) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	r, err := Initialize(ctx)

	defer func() {
		log.Fatal(r.Stop())
	}()

	return r.Run()
}

type repairer struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Initialize a repairer struct
func Initialize(ctx context.Context) (*repairer, error) {
	ctx, cancel := context.WithCancel(ctx)
	return &repairer{ctx: ctx, cancel: cancel}, nil
}

// Run the repairer loop
func (r *repairer) Run() (err error) {
	for {
		if err = r.ctx.Err(); err != nil {
			if err == context.Canceled {
				return nil
			}

			return err
		}
	}
}

// Stop the repairer loop
func (r *repairer) Stop() (err error) {
	r.cancel()
	return nil
}
