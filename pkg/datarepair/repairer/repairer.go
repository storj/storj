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

// Repairer is the interface for the data repair queue
type Repairer interface {
	Repair(seg *pb.InjuredSegment) error
	Run() error
	Stop() error
}

// Config contains configurable values for repairer
type Config struct {
	// queueAddress string `help:"data repair queue address" default:"localhost:7779"`
	maxRepair int `help:"maximum segments that can be repaired concurrently" default:"100"`
}

// Initialize a repairer struct
func (c *Config) Initialize(ctx context.Context) (Repairer, error) {
	var r repairer
	r.ctx, r.cancel = context.WithCancel(ctx)

	// TODO: Setup queue with c.queueAddress r.queue = queue

	r.cond.L = &r.mu
	r.maxRepair = c.maxRepair
	return &r, nil
}

// Run runs the checker with configured values
func (c *Config) Run(ctx context.Context) (err error) {
	r, err := c.Initialize(ctx)
	if err != nil {
		return err
	}

	return r.Run()
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

func (r *repairer) combinedError() error {
	if len(r.errs) == 0 {
		return nil
	}
	// TODO: combine errors
	return r.errs[0]
}
