// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"
	"strconv"

	"github.com/skyrings/skyring-common/tools/uuid"
	"go.uber.org/zap"

	"storj.io/storj/storage"
	"storj.io/storj/storage/redis"
)

type redisLiveAccounting struct {
	log *zap.Logger

	client *redis.Client
}

func newRedisLiveAccounting(log *zap.Logger, address string) (*redisLiveAccounting, error) {
	client, err := redis.NewClientFrom(address)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &redisLiveAccounting{
		log:    log,
		client: client,
	}, nil
}

// GetProjectStorageUsage gets inline and remote storage totals for a given
// project, back to the time of the last accounting tally.
func (cache *redisLiveAccounting) GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (totalUsed int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)
	val, err := cache.client.Get(ctx, projectID[:])
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return 0, nil
		}
		return 0, Error.Wrap(err)
	}
	intval, err := strconv.Atoi(string(val))
	return int64(intval), Error.Wrap(err)
}

// AddProjectStorageUsage lets the live accounting know that the given
// project has just added spaceUsed
func (cache *redisLiveAccounting) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, spaceUsed int64) (err error) {
	defer mon.Task()(&ctx, projectID, spaceUsed)(&err)
	return cache.client.IncrBy(ctx, projectID[:], spaceUsed)
}

// ResetTotals reset all space-used totals for all projects back to zero. This
// would normally be done in concert with calculating new tally counts in the
// accountingDB.
func (cache *redisLiveAccounting) ResetTotals(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	cache.log.Debug("Resetting real-time accounting data")
	return cache.client.FlushDB()
}

func (cache *redisLiveAccounting) GetAllProjectTotals(ctx context.Context) (_ map[uuid.UUID]int64, err error) {
	defer mon.Task()(&ctx)(&err)

	projects := make(map[uuid.UUID]int64, 0)

	err = cache.client.Iterate(ctx, storage.IterateOptions{Recurse: true}, func(ctx context.Context, it storage.Iterator) error {
		var item storage.ListItem
		for it.Next(ctx, &item) {
			if item.Key == nil {
				panic("nil key")
			}
			id := new(uuid.UUID)
			copy(id[:], item.Key[:])
			intval, err := strconv.Atoi(string(item.Value))
			if err != nil {
				panic("could not get project total")
			}
			projects[*id] = int64(intval)
		}
		return err
	})
	return projects, err
}

// Close the DB connection.
func (cache *redisLiveAccounting) Close() error {
	return cache.client.Close()
}
