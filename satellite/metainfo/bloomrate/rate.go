// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomrate

import (
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// Rate is a token bucket rate limiter that uses a single uint64 word. The
// 10 least significant bits represent tokens that allow for burst behavior.
// The remaining 54 significant bits are a timestamp (treated as positive
// nanoseconds from Unix epoch, with the 9 least significant bits masked off,
// for a loss of granularity of about half a microsecond). This rate limiter
// cannot be configured to allow rates higher than 1,956,947 events/second.
// This timestamp is when the current tokens were modified. The expiry is bumped
// every time the tokens are changed.
// The zero value is valid and works as if no traffic has yet arrived.
// Note that the burst logic is not exact - there might be a slightly higher
// amount of requests let through than the provided burst limit, but not in
// the long term.
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
	// how much should the expiry get bumped for each token?
	timePerOperation := time.Duration(float64(time.Second) / float64(limit))

	// parse the value
	val := (*atomic.Uint64)(r).Load()
	expiry := time.Unix(0, int64((val&timeMask)>>1))
	tokens := int(val & burstMask)

	// fill token bucket
	if now.Sub(expiry) > timePerOperation {
		tokens = min(tokens+int(now.Sub(expiry)/timePerOperation), burst)

		// calculate new expiry in timePerOperation steps to reduce incremental error
		if tokens == burst {
			expiry = now
		} else {
			expiry = expiry.Add(time.Duration(timePerOperation.Nanoseconds() * (now.Sub(expiry) / timePerOperation).Nanoseconds()))
		}
	}
	if tokens == 0 {
		return false
	}

	if (*atomic.Uint64)(r).CompareAndSwap(val,
		uint64(tokens-1)|((uint64(expiry.UnixNano())<<1)&timeMask)) {
		// no one raced us. it's done!
		return true
	}

	// someone raced us. this means there's a new timestamp in there. is there
	// more than 1 token? it's too bad CompareAndSwap doesn't
	// return the value you raced with.
	if tokens <= 1 {
		// no, someone else used our token
		return false
	}

	// we are going to just store the expiry and the tokens indiscriminately and hope for the best.
	(*atomic.Uint64)(r).Store(uint64(tokens-2) | ((uint64(expiry.UnixNano()) << 1) & timeMask))
	return true
}
