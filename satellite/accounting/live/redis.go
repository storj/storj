// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"
	"encoding/json"

	"github.com/skyrings/skyring-common/tools/uuid"
	"go.uber.org/zap"

	"storj.io/storj/storage"
	"storj.io/storj/storage/redis"
)

type redisLiveAccounting struct {
	log *zap.Logger

	client *redis.Client
}

func newRedisLiveAccounting(log *zap.Logger, address string) (Service, error) {
	client, err := redis.NewClientFrom(address)
	if err != nil {
		return nil, err
	}
	return &redisLiveAccounting{
		log:    log,
		client: client,
	}, nil
}

// GetProjectStorageUsage gets inline and remote storage totals for a given
// project, back to the time of the last accounting tally.
func (cache *redisLiveAccounting) GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (inlineTotal, remoteTotal int64, err error) {
	marshalled, err := cache.client.Get(ctx, []byte(projectID.String()))
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	var curVal spaceUsedAccounting
	err = json.Unmarshal(marshalled, &curVal)
	if err != nil {
		return 0, 0, err
	}
	return curVal.InlineSpace, curVal.RemoteSpace, nil
}

// AddProjectStorageUsage lets the live accounting know that the given
// project has just added InlineSpaceUsed bytes of inline space usage
// and RemoteSpaceUsed bytes of remote space usage.
func (cache *redisLiveAccounting) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, inlineSpaceUsed, remoteSpaceUsed int64) error {
	curInlineTotal, curRemoteTotal, err := cache.GetProjectStorageUsage(ctx, projectID)
	if err != nil {
		return err
	}
	totalSpaceUsed := spaceUsedAccounting{
		InlineSpace: curInlineTotal + inlineSpaceUsed,
		RemoteSpace: curRemoteTotal + remoteSpaceUsed,
	}
	marshalled, err := json.Marshal(totalSpaceUsed)
	if err != nil {
		return err
	}
	return cache.client.Put(ctx, []byte(projectID.String()), marshalled)
}

// ResetTotals reset all space-used totals for all projects back to zero. This
// would normally be done in concert with calculating new tally counts in the
// accountingDB.
func (cache *redisLiveAccounting) ResetTotals(ctx context.Context) error {
	cache.log.Info("Resetting real-time accounting data")
	return cache.client.FlushDB()
}
