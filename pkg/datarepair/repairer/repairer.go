// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"fmt"

	"gopkg.in/spacemonkeygo/monkit.v2"

	q "storj.io/storj/pkg/datarepair/queue"
)

var (
	mon = monkit.Package()
)

type repairer struct {
	ctx    context.Context
	cancel context.CancelFunc
	queue  q.RepairQueue
}

// Initialize a repairer struct
func Initialize(ctx context.Context, queue q.RepairQueue) (*repairer, error) {
	ctx, cancel := context.WithCancel(ctx)
	return &repairer{ctx: ctx, cancel: cancel, queue: queue}, nil
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
