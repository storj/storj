// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
	"storj.io/storj/storage/redis"
)

// Config contains configurable values for checker
type Config struct {
	QueueAddress string        `help:"data checker queue address" default:"redis://127.0.0.1:6378?db=1&password=abc123"`
	Interval     time.Duration `help:"how frequently checker should audit segments" default:"30s"`
}

// Initialize a Checker struct
func (c Config) initialize(ctx context.Context) (Checker, error) {
	pdb := pointerdb.LoadFromContext(ctx)
	if pdb == nil {
		return nil, Error.New("failed to load pointerdb from context")
	}

	sdb, ok := ctx.Value("masterdb").(interface {
		StatDB() statdb.DB
	})
	if !ok {
		return nil, Error.New("unable to get master db instance")
	}

	db, ok := ctx.Value("masterdb").(interface {
		Irreparable() irreparable.DB
	})
	if !ok {
		return nil, Error.New("unable to get master db instance")
	}
	o := overlay.LoadServerFromContext(ctx)

	driver, _, err := utils.SplitDBURL(c.QueueAddress)
	if err != nil {
		return nil, err
	}

	var repairQueue *queue.Queue
	switch driver {
	case "postgres":
		postgresDB, ok := ctx.Value("masterdb").(interface {
			RepairQueueDB() storage.Queue
		})
		if !ok {
			return nil, errs.New("unable to get master db instance")
		}
		repairQueue = queue.NewQueue(postgresDB.RepairQueueDB())
	case "redis":
		redisDB, err := redis.NewQueueFrom(c.QueueAddress)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		repairQueue = queue.NewQueue(redisDB)
	default:
		return nil, errs.New("Unsupported driver (%s). Please use postgres or redis", driver)
	}
	repairQueue := queue.NewQueue(redisQ)
	return newChecker(pdb, sdb.StatDB(), repairQueue, o, db.Irreparable(), 0, zap.L(), c.Interval), nil
}

// Run runs the checker with configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	check, err := c.initialize(ctx)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		if err := check.Run(ctx); err != nil {
			defer cancel()
			zap.L().Error("Error running checker", zap.Error(err))
		}
	}()

	return server.Run(ctx)
}
