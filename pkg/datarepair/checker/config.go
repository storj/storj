// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"storj.io/storj/pkg/provider"
)

// Config contains configurable values for repairer
type Config struct {
	QueueAddress string        `help:"data repair queue address" default:"redis://localhost:6379?db=5&password=123"`
	Interval     time.Duration `help:"how frequently checker should audit segments" default:"30s"`
}

// Initialize a Checker struct
func (c Config) initialize(ctx context.Context) (Checker, error) {
	// var check checker
	// check.ctx, check.cancel = context.WithCancel(ctx)
	check := newChecker(pointerdb *pointerdb.Server, repairQueue *queue.Queue, overlay pb.OverlayServer, limit int, logger *zap.Logger)

	// client, err := redis.NewClientFrom(c.QueueAddress)
	// if err != nil {
	// 	return nil, Error.Wrap(err)
	// }
	// r.queue = q.NewQueue(client)

	// r.cond.L = &r.mu
	// r.maxRepair = c.MaxRepair
	// r.interval = c.Interval
	// return &r, nil
	return &checker{}, nil
}

// Run runs the checker with configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	check, err := c.initialize(ctx)
	if err != nil {
		return err
	}
	return check.Run()
}
