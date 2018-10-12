// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	q "storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/utils"
)

// Repairer is the interface for the data repair queue
type Repairer interface {
	Repair(seg *pb.InjuredSegment) error
	Run() error
	Stop() error
}

// repairer holds important values for data repair
type repairer struct {
	ctx        context.Context
	cancel     context.CancelFunc
	queue      q.RepairQueue
	errs       []error
	mu         sync.Mutex
	cond       sync.Cond
	maxRepair  int
	inProgress int
	interval   time.Duration
}

// Run the repairer loop
func (r *repairer) Run() (err error) {
	zap.S().Info("Repairer is starting up")

	c := make(chan *pb.InjuredSegment)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
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
			return utils.CombineErrors(r.errs...)
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
func (r *repairer) Repair(seg *pb.InjuredSegment) (err error) {
	defer mon.Task()(&r.ctx)(&err)
	r.inProgress++
	fmt.Println(seg)

	r.inProgress--
	r.cond.Signal()
	return err
}

// Stop the repairer loop
func (r *repairer) Stop() (err error) {
	r.cancel()
	return nil
}
