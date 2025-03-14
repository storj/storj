// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomrate

import (
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func assert(t *testing.T, ok bool) {
	t.Helper()
	if !ok {
		t.Fatal("assertion failed")
	}
}

func TestRate(t *testing.T) {
	var r Rate

	now := time.Now()
	// use up the burst rate
	assert(t, r.Allow(now, rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(time.Millisecond), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(2*time.Millisecond), rate.Every(time.Second), 3))

	// okay these should now get rejected
	assert(t, !r.Allow(now.Add(3*time.Millisecond), rate.Every(time.Second), 3))
	assert(t, !r.Allow(now.Add(4*time.Millisecond), rate.Every(time.Second), 3))
	assert(t, !r.Allow(now.Add(5*time.Millisecond), rate.Every(time.Second), 3))
	// should have filled after a second
	assert(t, r.Allow(now.Add(time.Second), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(2*time.Second), rate.Every(time.Second), 3))
	// shy a microsecond, should get rejected
	assert(t, !r.Allow(now.Add(3*time.Second-time.Microsecond), rate.Every(time.Second), 3))

	// the counter should have expired by 3 seconds. make sure many requests can
	// happen as long as they stay within the rate limit.
	assert(t, r.Allow(now.Add(3*time.Second), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(4*time.Second), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(5*time.Second), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(6*time.Second), rate.Every(time.Second), 3))

	// allow to fill then increase the rate and make sure we fail
	assert(t, r.Allow(now.Add(9*time.Second), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(9*time.Second+time.Second/3), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(9*time.Second+2*time.Second/3), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(10*time.Second), rate.Every(time.Second), 3))
	assert(t, !r.Allow(now.Add(10*time.Second+time.Second/3), rate.Every(time.Second), 3))

	// try a rate that is bursty but overall under the limit
	assert(t, r.Allow(now.Add(13*time.Second), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(13*time.Second+time.Second/2), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(15*time.Second), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(15*time.Second+time.Second/2), rate.Every(time.Second), 3))

	// make sure it caps us if we go over.
	assert(t, r.Allow(now.Add(18*time.Second), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(18*time.Second+time.Second/2), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(19*time.Second), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(19*time.Second+time.Second/2), rate.Every(time.Second), 3))
	assert(t, r.Allow(now.Add(20*time.Second), rate.Every(time.Second), 3))
	assert(t, !r.Allow(now.Add(20*time.Second+time.Second/2), rate.Every(time.Second), 3))
}
