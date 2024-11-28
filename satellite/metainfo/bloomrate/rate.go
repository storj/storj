// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomrate

import (
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// Rate is a rate limiter that uses a single uint64 word. The 10 least
// significant bits represent a counter that allows for burst behavior. As long
// as the current limiter hasn't expired, the counter is allowed to climb to
// the burst limit.
// The remaining 54 significant bits are a timestamp (treated as positive
// nanoseconds from Unix epoch, with the 9 least significant bits masked off,
// for a loss of granularity of about half a microsecond). This rate limiter
// cannot be configured to allow rates higher than 1,956,947 events/second.
// This timestamp is when the current counter expires. The expiry is bumped
// every time the counter is bumped.
// The zero value is valid and works as if no traffic has yet arrived.
// Note that the burst logic is not exact - there might be a slightly higher
// amount of requests let through than the provided burst limit, but not in
// the long term.
// Also note that this is not a token bucket design - the burst amount is
// allowed per time window, and the limit remains hit until the time window
// expires, at which point the burst limit is allowed through again. For this
// reason, smaller burst amounts are better than larger burst amounts.
type Rate atomic.Uint64

const (
	burstMask = (1 << 10) - 1
	timeMask  = ^uint64(0) - burstMask
)

// Allow takes now as a timestamp, the limit (events/second), and the burst
// amount, and returns whether the current rate allows it. It is intended
// that limit and burst are consistent and don't change across calls to Allow,
// but are stored elsewhere (see BloomRate).
func (r *Rate) Allow(now time.Time, limit rate.Limit, burst int) bool {
	// how much should the expiry get bumped if we bump the counter?
	timePerOperation := time.Duration(float64(time.Second) / float64(limit))

	// parse the value
	val := (*atomic.Uint64)(r).Load()
	expiry := time.Unix(0, int64((val&timeMask)>>1))

	// has the counter expired?
	if now.Before(expiry) {
		// no. is the count under the burst limit?
		if int(val&burstMask) < burst {
			// success. bump the expiry and the counter and allow this through.
			(*atomic.Uint64)(r).Add(1 | ((uint64(timePerOperation) << 1) & timeMask))
			// N.B. - we bumped the expiry and the counter. perhaps we could improve
			// this rate limiting algorithm if we checked the new return value to see
			// if anyone else raced with us.
			return true
		}
		// no, we've hit the counter limit.
		return false
	}

	// the counter is expired. let's store a brand new expiry and count of 1.
	if (*atomic.Uint64)(r).CompareAndSwap(val,
		1|((uint64(now.Add(timePerOperation).UnixNano())<<1)&timeMask)) {
		// no one raced us. it's done!
		return true
	}

	// someone raced us. this means there's a new timestamp in there. do bursts
	// allow for more than 1 at a time? it's too bad CompareAndSwap doesn't
	// return the value you raced with.
	if burst <= 1 {
		// no, we only do one event per expiry.
		return false
	}

	// we do more than one event per expiry. like before, we are going to just
	// bump the expiry and the counter indiscriminately and hope for the best.
	(*atomic.Uint64)(r).Add(1 | ((uint64(timePerOperation) << 1) & timeMask))
	return true
}
