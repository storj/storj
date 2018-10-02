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

// Repairer holds important values for data repair
type Repairer struct {
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
func Initialize(ctx context.Context, queue q.RepairQueue, max int) (*Repairer, error) {
	var r Repairer
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.queue = queue
	r.cond.L = &r.mu
	r.maxRepair = max
	return &r, nil
}

// Run the repairer loop
func (r *Repairer) Run() (err error) {
	c := make(chan *pb.InjuredSegment)
	go func() {
		for {
			for r.inProgress >= r.maxRepair {
				r.cond.Wait()
			}

			// GetNext should lock until there is an actual next item in the queue
			seg, err := r.queue.Dequeue()
			if err != nil {
				r.errs = append(r.errs, err)
				r.cancel()
			}
			c <- &seg
		}
	}()

	for {
		select {
		case <-r.ctx.Done():
			return r.combinedError()
		case seg := <-c:
			go func() {
				err := r.Repair(seg)
				if err != nil {
					r.errs = append(r.errs, err)
					r.cancel()
				}
			}()
		}
	}
}

// Repair starts repair of the segment
func (r *Repairer) Repair(seg *pb.InjuredSegment) (err error) {
	defer mon.Task()(&r.ctx)(&err)
	r.inProgress++
	fmt.Println(seg)

	r.inProgress--
	r.cond.Signal()
	return err
}

// Stop the repairer loop
func (r *Repairer) Stop() (err error) {
	r.cancel()
	return nil
}

func (r *Repairer) combinedError() error {
	if len(r.errs) == 0 {
		return nil
	}
	// TODO: combine errors
	return r.errs[0]
}
