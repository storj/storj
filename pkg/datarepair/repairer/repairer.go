// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"fmt"
	"sync"
	"time"

	q "storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage/redis"
)

// Repairer is the interface for the data repair queue
type Repairer interface {
	Repair(seg *pb.InjuredSegment) error
	Run() error
	Stop() error
}

// Config contains configurable values for repairer
type Config struct {
	QueueAddress string        `help:"data repair queue address" default:"redis://localhost:6379?db=5&password=123"`
	MaxRepair    int           `help:"maximum segments that can be repaired concurrently" default:"100"`
	Interval     time.Duration `help:"how frequently checker should audit segments" default:"3600s"`
}

// Initialize a repairer struct
func (c Config) initialize(ctx context.Context) (Repairer, error) {
	var r repairer
	r.ctx, r.cancel = context.WithCancel(ctx)

	client, err := redis.NewClientFrom(c.QueueAddress)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	r.queue = q.NewQueue(client)

	r.cond.L = &r.mu
	r.maxRepair = c.MaxRepair
	r.interval = c.Interval
	return &r, nil
}

// Run runs the repairer with configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	r, err := c.initialize(ctx)
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
	interval   time.Duration
}

// Run the repairer loop
func (r *repairer) Run() (err error) {
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
