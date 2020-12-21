// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
)

type redisLiveAccounting struct {
	client *redis.Client
}

// newRedisLiveAccounting returns a redisLiveAccounting cache instance.
//
// It returns accounting.ErrInvalidArgument if the connection address is invalid
// according to Redis.
//
// The function pings to the Redis server for verifying the connectivity but if
// it fails then it returns an instance and accounting.ErrSystemOrNetError
// because it means that Redis may not be operative at this precise moment but
// it may be in future method calls as it handles automatically reconnects.
func newRedisLiveAccounting(address string) (*redisLiveAccounting, error) {
	redisurl, err := url.Parse(address)
	if err != nil {
		return nil, accounting.ErrInvalidArgument.New("address: invalid URL; %w", err)
	}

	if redisurl.Scheme != "redis" {
		return nil, accounting.ErrInvalidArgument.New("address: not a redis:// formatted address")
	}

	q := redisurl.Query()
	db := q.Get("db")
	if db == "" {
		return nil, accounting.ErrInvalidArgument.New("address: a database number has to be specified")
	}

	dbn, err := strconv.Atoi(db)
	if err != nil {
		return nil, accounting.ErrInvalidArgument.New("address: invalid database number %s", db)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     redisurl.Host,
		Password: q.Get("password"),
		DB:       dbn,
	})

	cache := &redisLiveAccounting{
		client: client,
	}

	// ping here to verify we are able to connect to Redis with the initialized client.
	if err := client.Ping().Err(); err != nil {
		return cache, accounting.ErrSystemOrNetError.New("Redis ping failed: %w", err)
	}

	return cache, nil
}

// GetProjectStorageUsage gets inline and remote storage totals for a given
// project, back to the time of the last accounting tally.
func (cache *redisLiveAccounting) GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (totalUsed int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	return cache.getInt64(ctx, string(projectID[:]))
}

// createBandwidthProjectIDKey creates the bandwidth project key.
// The current month is combined with projectID to create a prefix.
func createBandwidthProjectIDKey(projectID uuid.UUID, now time.Time) string {
	// Add current month as prefix
	_, month, _ := now.Date()
	key := append(projectID[:], byte(int(month)))

	return string(key) + ":bandwidth"
}

// GetProjectBandwidthUsage returns the current bandwidth usage
// from specific project.
func (cache *redisLiveAccounting) GetProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, now time.Time) (currentUsed int64, err error) {
	defer mon.Task()(&ctx, projectID, now)(&err)

	return cache.getInt64(ctx, createBandwidthProjectIDKey(projectID, now))
}

// UpdateProjectBandwidthUsage increment the bandwidth cache key value.
func (cache *redisLiveAccounting) UpdateProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, increment int64, ttl time.Duration, now time.Time) (err error) {
	mon.Task()(&ctx, projectID, increment, ttl, now)(&err)

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
	err = cache.client.Eval(script, []string{key}).Err()
	if err != nil {
		return accounting.ErrSystemOrNetError.New("Redis eval failed: %w", err)
	}

	return nil
}

// AddProjectStorageUsage lets the live accounting know that the given
// project has just added spaceUsed bytes of storage (from the user's
// perspective; i.e. segment size).
func (cache *redisLiveAccounting) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, spaceUsed int64) (err error) {
	defer mon.Task()(&ctx, projectID, spaceUsed)(&err)

	_, err = cache.client.IncrBy(string(projectID[:]), spaceUsed).Result()
	if err != nil {
		return accounting.ErrSystemOrNetError.New("Redis incrby failed: %w", err)
	}

	return nil
}

// GetAllProjectTotals iterates through the live accounting DB and returns a map of project IDs and totals.
func (cache *redisLiveAccounting) GetAllProjectTotals(ctx context.Context) (_ map[uuid.UUID]int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var (
		seen     = make(map[string]struct{})
		projects = make(map[uuid.UUID]int64)
	)

	it := cache.client.Scan(0, "*", 0).Iterator()
	for it.Next() {
		key := it.Val()
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		// skip bandwidth keys
		if strings.HasSuffix(key, "bandwidth") {
			continue
		}

		projectID, err := uuid.FromBytes([]byte(key))
		if err != nil {
			return nil, accounting.ErrUnexpectedValue.New("cannot parse the key as UUID; key=%q", key)
		}

		val, err := cache.getInt64(ctx, key)
		if err != nil {
			if accounting.ErrKeyNotFound.Has(err) {
				continue
			}

			return nil, err
		}

		projects[projectID] = val
	}

	return projects, nil
}

// Close the DB connection.
func (cache *redisLiveAccounting) Close() error {
	err := cache.client.Close()
	if err != nil {
		return accounting.ErrSystemOrNetError.New("Redis close failed: %w", err)
	}

	return nil
}

func (cache *redisLiveAccounting) getInt64(ctx context.Context, key string) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	val, err := cache.client.Get(key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, accounting.ErrKeyNotFound.New("%q", key)
		}

		return 0, accounting.ErrSystemOrNetError.New("Redis get failed: %w", err)
	}

	intval, err := strconv.ParseInt(string(val), 10, 64)
	if err != nil {
		return 0, accounting.ErrUnexpectedValue.New("cannot parse the value as int64; key=%q val=%q", key, val)
	}

	return intval, nil
}
