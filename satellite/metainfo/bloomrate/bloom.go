// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomrate

import (
	"hash/maphash"
	"time"

	"golang.org/x/time/rate"
)

// BloomRate is kind of like a count-min datastructure, kind of like a bloom
// filter. The problem it solves is rate limiting a huge number of event types
// independently, whereas it is okay if occasionally two event types collide and
// share the same rate limiter. The way it works is there is a large array of
// rate limiters, and when an event type is rate limited, the event is
// consistently hashed into the rate limiter array a few times, and if any of
// the rate limiters allow the event to occur, it occurs.
type BloomRate struct {
	rates  []Rate
	hashes []maphash.Seed
	limit  rate.Limit
	burst  int
}

// NewBloomRate creates a BloomRate with 2^`size` rate limits and `hashes`
// hashing functions. Rate limiting uses `limit` (events/sec) and `burst`
// parameters, see the documentation on Rate.
func NewBloomRate(size int, hashes int, limit rate.Limit, burst int) *BloomRate {
	hashSeeds := make([]maphash.Seed, 0, hashes)
	for i := 0; i < hashes; i++ {
		hashSeeds = append(hashSeeds, maphash.MakeSeed())
	}
	return &BloomRate{
		rates:  make([]Rate, 1<<size),
		hashes: hashSeeds,
		limit:  limit,
		burst:  burst,
	}
}

// Allow takes now as a timestamp and returns whether the current rate allows
// an event at that time, updating rate limits as appropriate to indicate that
// the event happened.
func (br *BloomRate) Allow(now time.Time, key []byte) bool {
	var mh maphash.Hash
	allowed := false
	for _, seed := range br.hashes {
		mh.SetSeed(seed)
		_, _ = mh.Write(key)
		if br.rates[int(mh.Sum64()&uint64(len(br.rates)-1))].Allow(now, br.limit, br.burst) {
			allowed = true
			// we can't break out of the for loop here. we need to make sure all
			// rate limits are updated, as the first rate limit we find might have
			// high contention with another key, and a later rate limit might not.
		}
	}
	return allowed

}
