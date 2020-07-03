// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"storj.io/common/uuid"
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
	intval, err := strconv.ParseInt(string([]byte(val)), 10, 64)
	return intval, Error.Wrap(err)
}

// createBandwidthProjectIDKey creates the bandwidth project key.
// The current month is combined with projectID to create a prefix.
func createBandwidthProjectIDKey(projectID uuid.UUID, now time.Time) []byte {
	// Add current month as prefix
	_, month, _ := now.Date()
	key := append(projectID[:], byte(int(month)))

	return append(key, []byte(":bandwidth")...)
}

// GetProjectBandwidthUsage returns the current bandwidth usage
// from specific project.
func (cache *redisLiveAccounting) GetProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, now time.Time) (currentUsed int64, err error) {
	val, err := cache.client.Get(ctx, createBandwidthProjectIDKey(projectID, now))
	if err != nil {
		return 0, err
	}
	intval, err := strconv.ParseInt(string([]byte(val)), 10, 64)
	return intval, Error.Wrap(err)
}

// UpdateProjectBandwidthUsage increment the bandwidth cache key value.
func (cache *redisLiveAccounting) UpdateProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, increment int64, ttl time.Duration, now time.Time) (err error) {

	// The following script will increment the cache key
	// by a specific value. If the key does not exist, it is
	// set to 0 before performing the operation.
	// The key expiration will be set only in the first iteration.
	// To achieve this we compare the increment and key value,
	// if they are equal its the first iteration.
	// More details on rate limiter section: https://redis.io/commands/incr

	script := fmt.Sprintf(`local current
	current = redis.call("incrby", KEYS[1], "%d")
	if tonumber(current) == "%d" then
		redis.call("expire",KEYS[1], %d)
	end
	return current
	`, increment, increment, int(ttl.Seconds()))

	key := createBandwidthProjectIDKey(projectID, now)

	return cache.client.Eval(ctx, script, []string{string(key)})
}

// AddProjectStorageUsage lets the live accounting know that the given
// project has just added spaceUsed bytes of storage (from the user's
// perspective; i.e. segment size).
func (cache *redisLiveAccounting) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, spaceUsed int64) (err error) {
	defer mon.Task()(&ctx, projectID, spaceUsed)(&err)
	return cache.client.IncrBy(ctx, projectID[:], spaceUsed)
}

// GetAllProjectTotals iterates through the live accounting DB and returns a map of project IDs and totals.
func (cache *redisLiveAccounting) GetAllProjectTotals(ctx context.Context) (_ map[uuid.UUID]int64, err error) {
	defer mon.Task()(&ctx)(&err)

	projects := make(map[uuid.UUID]int64)

	err = cache.client.Iterate(ctx, storage.IterateOptions{Recurse: true}, func(ctx context.Context, it storage.Iterator) error {
		var item storage.ListItem
		for it.Next(ctx, &item) {
			if item.Key == nil {
				return Error.New("nil key")
			}
			id := new(uuid.UUID)
			copy(id[:], item.Key[:])
			intval, err := strconv.ParseInt(string([]byte(item.Value)), 10, 64)
			if err != nil {
				return Error.New("could not get total for project %s", id.String())
			}
			if !strings.HasSuffix(item.Key.String(), "bandwidth") {
				projects[*id] = intval
			}
		}
		return nil
	})
	return projects, err
}

// Close the DB connection.
func (cache *redisLiveAccounting) Close() error {
	return cache.client.Close()
}
