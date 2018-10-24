// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"time"

	q "storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/storage/redis"
)

// Config contains configurable values for repairer
type Config struct {
	QueueAddress string        `help:"data repair queue address" default:"redis://localhost:6379?db=0&password=testpass"`
	MaxRepair    int           `help:"maximum segments that can be repaired concurrently" default:"100"`
	Interval     time.Duration `help:"how frequently checker should audit segments" default:"3600s"`
}

// Run runs the repairer with configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	client, err := redis.NewClientFrom(c.QueueAddress)
	if err != nil {
		return Error.Wrap(err)
	}

	queue := q.NewQueue(client)

	repairer := newRepairer(ctx, queue, c.Interval, c.MaxRepair)
	return repairer.Run()
}
