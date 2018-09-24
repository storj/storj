// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"fmt"
	"log"

	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/datarepair"
)

var (
	mon = monkit.Package()
)

// Config contains configurable values for repairer
type Config struct {
	queue datarepair.RepairQueue
}

// Run runs the repairer with configured values
func (c *Config) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	r, err := Initialize(ctx, c)

	defer func() {
		log.Fatal(r.Stop())
	}()

	return r.Run()
}

type repairer struct {
	ctx    context.Context
	cancel context.CancelFunc
	queue  datarepair.RepairQueue
}

// Initialize a repairer struct
func Initialize(ctx context.Context, config *Config) (*repairer, error) {
	ctx, cancel := context.WithCancel(ctx)
	return &repairer{ctx: ctx, cancel: cancel, queue: config.queue}, nil
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
		injuredSegment := r.queue.GetNext()
		fmt.Println(injuredSegment)

	}
	return nil
}

// Stop the repairer loop
func (r *repairer) Stop() (err error) {
	r.cancel()
	return nil
}
