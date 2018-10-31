// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"time"

	"go.uber.org/zap"
	q "storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/storage/redis"
)

// Config contains configurable values for repairer
type Config struct {
	QueueAddress string        `help:"data repair queue address" default:"redis://127.0.0.1:6378?db=1&password=abc123"`
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

	repairer := newRepairer(queue, c.Interval, c.MaxRepair)

	// TODO(coyle): we need to figure out how to propagate the error up to cancel the service
	go func() {
		if err := repairer.Run(ctx); err != nil {
			zap.L().Error("Error running repairer", zap.Error(err))
		}
	}()

	return server.Run(ctx)
}
