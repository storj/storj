// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package datarepair

import (
	"context"

	"gopkg.in/spacemonkeygo/monkit.v2"

	q "storj.io/storj/pkg/datarepair/queue"
	// "storj.io/storj/pkg/datarepair/checker"
	"storj.io/storj/pkg/datarepair/repairer"
)

var (
	mon = monkit.Package()
)

// Config contains configurable values for repairer
type Config struct {
	maxRepair int
	//TODO: Add things for checker
	//TODO: Add things for repairer
}

// Run runs the repairer with configured values
func (c *Config) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var queue q.RepairQueue

	// TODO: Initialize Checker with queue

	// Initialize Repairer with queue
	_, err = repairer.Initialize(ctx, queue, c.maxRepair)
	if err != nil {
		return err
	}

	// TODO: Run the Checker in goroutine
	// TODO: Run the Repairer in goroutine

	// TODO: defer stop of checker and repairer

	return err
}
