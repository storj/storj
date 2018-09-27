// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"fmt"
	"sync"

	"gopkg.in/spacemonkeygo/monkit.v2"

	q "storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/pb"
)

var (
	mon = monkit.Package()
)

type repairer struct {
	ctx        context.Context
	cancel     context.CancelFunc
	queue      q.RepairQueue
	errs       []error
	mu         sync.Mutex
	cond       sync.Cond
	maxRepair  int
	inProgress int
}

// Initialize a repairer struct
func Initialize(ctx context.Context, queue q.RepairQueue) (*repairer, error) {
	var r repairer
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.queue = queue
	r.cond.L = &r.mu
	r.maxRepair = 5
	return &r, nil
}

// Run the repairer loop
func (r *repairer) Run() (err error) {
	c := make(chan *pb.InjuredSegment)
	go func() {
		for {
			for r.inProgress >= r.maxRepair {
				r.cond.Wait()
			}

			// GetNext should lock until there is an actual next item in the queue
			_, seg, err := r.queue.GetNext()
			if err != nil {
				r.errs = append(r.errs, err)
				r.cancel()
			}
			c <- seg
		}
	}()

	for {
		select {
		case <-r.ctx.Done():
			return r.combinedError()
		case seg := <-c:
			go r.Repair(seg)
		}
	}

	return nil
}

func (r *repairer) Repair(seg *pb.InjuredSegment) {
	r.inProgress += 1
	fmt.Println(seg)

	r.inProgress -= 1
	r.cond.Signal()
}

// Stop the repairer loop
func (r *repairer) Stop() (err error) {
	r.cancel()
	return nil
}

func (r *repairer) combinedError() error {
	if len(r.errs) == 0 {
		return nil
	}
	// TODO: combine errors
	return r.errs[0]
}
