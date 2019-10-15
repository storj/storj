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
	val, err := cache.client.Get(ctx, []byte(projectID.String()))
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
// project has just added inlineSpaceUsed bytes of inline space usage
// and remoteSpaceUsed bytes of remote space usage.
func (cache *redisLiveAccounting) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, inlineSpaceUsed, remoteSpaceUsed int64) (err error) {
	defer mon.Task()(&ctx, projectID, inlineSpaceUsed, remoteSpaceUsed)(&err)
	if inlineSpaceUsed < 0 || remoteSpaceUsed < 0 {
		return Error.New("Used space amounts must be greater than 0. Inline: %d, Remote: %d", inlineSpaceUsed, remoteSpaceUsed)
	}
	return cache.client.IncrBy(ctx, []byte(projectID.String()), inlineSpaceUsed+remoteSpaceUsed)
}

// ResetTotals reset all space-used totals for all projects back to zero. This
// would normally be done in concert with calculating new tally counts in the
// accountingDB.
func (cache *redisLiveAccounting) ResetTotals(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	cache.log.Debug("Resetting real-time accounting data")
	return cache.client.FlushDB()
}

// Close the DB connection.
func (cache *redisLiveAccounting) Close() error {
	return cache.client.Close()
}
