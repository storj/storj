// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
)

// ErrGetProjectLimitCache error for getting project limits from cache.
var ErrGetProjectLimitCache = errs.Class("get project limits from cache")

type redisLiveAccounting struct {
	client *redis.Client

	batchSize int
}

// openRedisLiveAccounting returns a redisLiveAccounting cache instance.
//
// It returns accounting.ErrInvalidArgument if the connection address is invalid
// according to Redis.
//
// The function pings to the Redis server for verifying the connectivity but if
// it fails then it returns an instance and accounting.ErrSystemOrNetError
// because it means that Redis may not be operative at this precise moment but
// it may be in future method calls as it handles automatically reconnects.
func openRedisLiveAccounting(ctx context.Context, address string, batchSize int) (*redisLiveAccounting, error) {
	opts, err := redis.ParseURL(address)
	if err != nil {
		return nil, accounting.ErrInvalidArgument.Wrap(err)
	}

	cache := &redisLiveAccounting{
		client:    redis.NewClient(opts),
		batchSize: batchSize,
	}

	// ping here to verify we are able to connect to Redis with the initialized client.
	if err := cache.client.Ping(ctx).Err(); err != nil {
		return cache, accounting.ErrSystemOrNetError.New("Redis ping failed: %w", err)
	}

	return cache, nil
}

// GetProjectStorageUsage gets inline and remote storage totals for a given
// project, back to the time of the last accounting tally.
func (cache *redisLiveAccounting) GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (totalUsed int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	return cache.getInt64(ctx, createStorageProjectIDKey(projectID))
}

// GetProjectStorageAndSegmentUsage gets storage and segment usage for give project.
func (cache *redisLiveAccounting) GetProjectStorageAndSegmentUsage(ctx context.Context, projectID uuid.UUID) (storage, segments int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)

	storageKey := createStorageProjectIDKey(projectID)
	segmentsKey := createSegmentProjectIDKey(projectID)
	resultSlice := cache.client.MGet(ctx, storageKey, segmentsKey)
	results, err := resultSlice.Result()
	if err != nil {
		return 0, 0, accounting.ErrSystemOrNetError.New("Redis get failed: %w", err)
	}

	if len(results) != 2 {
		return 0, 0, accounting.ErrUnexpectedValue.New("wrong number of results: got %d wants %d", len(results), 2)
	}

	if results[0] != nil {
		storageValue := results[0].(string)
		storage, err = strconv.ParseInt(storageValue, 10, 64)
		if err != nil {
			err = accounting.ErrUnexpectedValue.New("cannot parse the value as int64; key=%q val=%q", storageKey, storageValue)
		}
	}

	if results[1] != nil {
		segmentsValue := results[1].(string)
		segments, err = strconv.ParseInt(segmentsValue, 10, 64)
		if err != nil {
			err = errs.Combine(err, accounting.ErrUnexpectedValue.New("cannot parse the value as int64; key=%q val=%q", segmentsKey, segmentsValue))
		}
	}

	return storage, segments, err
}

// GetProjectBandwidthUsage returns the current bandwidth usage
// from specific project.
func (cache *redisLiveAccounting) GetProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, now time.Time) (currentUsed int64, err error) {
	defer mon.Task()(&ctx, projectID, now)(&err)

	return cache.getInt64(ctx, createBandwidthProjectIDKey(projectID, now))
}

// InsertProjectBandwidthUsage inserts a project bandwidth usage if it
// doesn't exist. It returns true if it's inserted, otherwise false.
func (cache *redisLiveAccounting) InsertProjectBandwidthUsage(ctx context.Context, projectID uuid.UUID, value int64, ttl time.Duration, now time.Time) (inserted bool, err error) {
	mon.Task()(&ctx, projectID, value, ttl, now)(&err)

	// The following script will set the cache key to a specific value with an
	// expiration time to live when it doesn't exist, otherwise it ignores it.
	script := redis.NewScript(`local inserted
	inserted = redis.call("setnx", KEYS[1], ARGV[1])
	if tonumber(inserted) == 1 then
		redis.call("expire",KEYS[1], ARGV[2])
	end

	return inserted
	`)

	key := createBandwidthProjectIDKey(projectID, now)
	rcmd := script.Run(ctx, cache.client, []string{key}, value, int(ttl.Seconds()))
	if err != nil {
		return false, accounting.ErrSystemOrNetError.New("Redis eval failed: %w", err)
	}

	insert, err := rcmd.Int()
	if err != nil {
		err = accounting.ErrSystemOrNetError.New(
			"Redis script is invalid it must return an boolean. %w", err,
		)
	}

	return insert == 1, err
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
	script := redis.NewScript(`local current
	current = redis.call("incrby", KEYS[1], ARGV[1])
	if tonumber(current) == tonumber(ARGV[1]) then
		redis.call("expire", KEYS[1], ARGV[2])
	end
	return current
	`)

	key := createBandwidthProjectIDKey(projectID, now)
	err = script.Run(ctx, cache.client, []string{key}, increment, int(ttl.Seconds())).Err()
	if err != nil {
		return accounting.ErrSystemOrNetError.New("Redis eval failed: %w", err)
	}

	return nil
}

// AddProjectSegmentUsageUpToLimit increases segment usage up to the limit.
// If the limit is exceeded, the usage is not increased and accounting.ErrProjectLimitExceeded is returned.
func (cache *redisLiveAccounting) AddProjectSegmentUsageUpToLimit(ctx context.Context, projectID uuid.UUID, increment int64, segmentLimit int64) (err error) {
	defer mon.Task()(&ctx, projectID, increment)(&err)

	key := createSegmentProjectIDKey(projectID)

	// do a blind increment and checking the limit afterwards,
	// so that the success path has only one round-trip.
	newSegmentUsage, err := cache.client.IncrBy(ctx, key, increment).Result()
	if err != nil {
		return accounting.ErrSystemOrNetError.New("Redis incrby failed: %w", err)
	}

	if newSegmentUsage > segmentLimit {
		// roll back
		_, err = cache.client.DecrBy(ctx, key, increment).Result()
		if err != nil {
			return accounting.ErrSystemOrNetError.New("Redis decrby failed: %w", err)
		}

		return accounting.ErrProjectLimitExceeded.New("Additional %d segments exceed project limit of %d", increment, segmentLimit)
	}

	return nil
}

// AddProjectStorageUsageUpToLimit increases storage usage up to the limit.
// If the limit is exceeded, the usage is not increased and accounting.ErrProjectLimitExceeded is returned.
func (cache *redisLiveAccounting) AddProjectStorageUsageUpToLimit(ctx context.Context, projectID uuid.UUID, increment int64, spaceLimit int64) (err error) {
	defer mon.Task()(&ctx, projectID, increment)(&err)

	// do a blind increment and checking the limit afterwards,
	// so that the success path has only one round-trip.
	newSpaceUsage, err := cache.client.IncrBy(ctx, string(projectID[:]), increment).Result()
	if err != nil {
		return accounting.ErrSystemOrNetError.New("Redis incrby failed: %w", err)
	}

	if newSpaceUsage > spaceLimit {
		// roll back
		_, err = cache.client.DecrBy(ctx, string(projectID[:]), increment).Result()
		if err != nil {
			return accounting.ErrSystemOrNetError.New("Redis decrby failed: %w", err)
		}

		return accounting.ErrProjectLimitExceeded.New("Additional storage of %d bytes exceeds project limit of %d", increment, spaceLimit)
	}

	return nil
}

// UpdateProjectStorageAndSegmentUsage increment the storage and segment cache key values.
func (cache *redisLiveAccounting) UpdateProjectStorageAndSegmentUsage(ctx context.Context, projectID uuid.UUID, storageIncrement, segmentIncrement int64) (err error) {
	mon.Task()(&ctx, projectID, storageIncrement, segmentIncrement)(&err)

	pipe := cache.client.Pipeline()

	storageKey := createStorageProjectIDKey(projectID)
	segmentKey := createSegmentProjectIDKey(projectID)
	storageIncr := pipe.IncrBy(ctx, storageKey, storageIncrement)
	segmentIncr := pipe.IncrBy(ctx, segmentKey, segmentIncrement)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return accounting.ErrSystemOrNetError.New("Redis pipeline exec failed: %w", err)
	}

	var errsList errs.Group

	_, err = storageIncr.Result()
	if err != nil {
		errsList.Add(accounting.ErrSystemOrNetError.New("Redis storage incrby failed: %w", err))
	}
	_, err = segmentIncr.Result()
	if err != nil {
		errsList.Add(accounting.ErrSystemOrNetError.New("Redis segment incrby failed: %w", err))
	}
	if errsList.Err() != nil {
		return errsList.Err()
	}

	return nil
}

// GetAllProjectTotals iterates through the live accounting DB and returns a map of project IDs and totals, amount of segments.
//
// TODO (https://storjlabs.atlassian.net/browse/IN-173): see if it possible to
// get key/value pairs with one single call.
func (cache *redisLiveAccounting) GetAllProjectTotals(ctx context.Context) (_ map[uuid.UUID]accounting.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	projects := make(map[uuid.UUID]accounting.Usage)

	it := cache.client.Scan(ctx, 0, "*", 0).Iterator()
	for it.Next(ctx) {
		key := it.Val()

		// skip bandwidth keys
		if strings.HasSuffix(key, "bandwidth") {
			continue
		}

		if strings.HasSuffix(key, "segment") {
			projectID, err := uuid.FromBytes([]byte(strings.TrimSuffix(key, ":segment")))
			if err != nil {
				return nil, accounting.ErrUnexpectedValue.New("cannot parse the key as UUID; key=%q", key)
			}

			projects[projectID] = accounting.Usage{}
		} else {
			projectID, err := uuid.FromBytes([]byte(key))
			if err != nil {
				return nil, accounting.ErrUnexpectedValue.New("cannot parse the key as UUID; key=%q", key)
			}

			projects[projectID] = accounting.Usage{}
		}
	}

	return cache.fillUsage(ctx, projects)
}

func (cache *redisLiveAccounting) fillUsage(ctx context.Context, projects map[uuid.UUID]accounting.Usage) (_ map[uuid.UUID]accounting.Usage, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(projects) == 0 {
		return nil, nil
	}

	projectIDs := make([]uuid.UUID, 0, cache.batchSize)
	segmentKeys := make([]string, 0, cache.batchSize)
	storageKeys := make([]string, 0, cache.batchSize)

	fetchProjectsUsage := func() error {
		if len(projectIDs) == 0 {
			return nil
		}

		segmentResult, err := cache.client.MGet(ctx, segmentKeys...).Result()
		if err != nil {
			return ErrGetProjectLimitCache.Wrap(err)
		}

		storageResult, err := cache.client.MGet(ctx, storageKeys...).Result()
		if err != nil {
			return ErrGetProjectLimitCache.Wrap(err)
		}

		// Note, because we are using a cache, it might be empty and not contain the
		// information we are looking for -- or they might be still empty for some reason.

		for i, projectID := range projectIDs {
			segmentsUsage, err := parseAnyAsInt64(segmentResult[i])
			if err != nil {
				return errs.Wrap(err)
			}

			storageUsage, err := parseAnyAsInt64(storageResult[i])
			if err != nil {
				return errs.Wrap(err)
			}

			projects[projectID] = accounting.Usage{
				Segments: segmentsUsage,
				Storage:  storageUsage,
			}
		}

		return nil
	}

	for projectID := range projects {
		projectIDs = append(projectIDs, projectID)
		segmentKeys = append(segmentKeys, createSegmentProjectIDKey(projectID))
		storageKeys = append(storageKeys, createStorageProjectIDKey(projectID))

		if len(projectIDs) >= cache.batchSize {
			err := fetchProjectsUsage()
			if err != nil {
				return nil, err
			}

			projectIDs = projectIDs[:0]
			segmentKeys = segmentKeys[:0]
			storageKeys = storageKeys[:0]
		}
	}

	err = fetchProjectsUsage()
	if err != nil {
		return nil, err
	}

	return projects, nil
}

func parseAnyAsInt64(v any) (int64, error) {
	if v == nil {
		return 0, nil
	}

	s, ok := v.(string)
	if !ok {
		return 0, accounting.ErrUnexpectedValue.New("cannot parse the value as int64; val=%q", v)
	}

	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, accounting.ErrUnexpectedValue.New("cannot parse the value as int64; val=%q", v)
	}

	return i, nil
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

	val, err := cache.client.Get(ctx, key).Bytes()
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

// createBandwidthProjectIDKey creates the bandwidth project key.
// The current month is combined with projectID to create a prefix.
func createBandwidthProjectIDKey(projectID uuid.UUID, now time.Time) string {
	// Add current month as prefix
	_, month, day := now.Date()
	return string(projectID[:]) + string(byte(month)) + string(byte(day)) + ":bandwidth"
}

// createSegmentProjectIDKey creates the segment project key.
func createSegmentProjectIDKey(projectID uuid.UUID) string {
	return string(projectID[:]) + ":segment"
}

// createStorageProjectIDKey creates the storage project key.
func createStorageProjectIDKey(projectID uuid.UUID) string {
	return string(projectID[:])
}
