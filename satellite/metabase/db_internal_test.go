// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/dbutil"
)

func TestLimitedAsOfSystemTime(t *testing.T) {
	const (
		unixNano    = 1623324728961910000
		unixNanoStr = `1623324728961910000`
	)

	check := func(expect string, startNano, baselineNano int64, maxInterval time.Duration) {
		var start, baseline time.Time
		if startNano != 0 {
			start = time.Unix(0, startNano)
		}
		if baselineNano != 0 {
			baseline = time.Unix(0, baselineNano)
		}
		result := metabase.LimitedAsOfSystemTime(dbutil.Cockroach, start, baseline, maxInterval)
		require.Equal(t, expect, result)
	}

	// baseline in the future
	check("",
		unixNano-time.Second.Nanoseconds(),
		unixNano,
		0,
	)

	// ignore interval when positive or zero
	check(" AS OF SYSTEM TIME '"+unixNanoStr+"' ",
		unixNano+time.Second.Nanoseconds(),
		unixNano,
		0,
	)
	check(" AS OF SYSTEM TIME '"+unixNanoStr+"' ",
		unixNano+time.Second.Nanoseconds(),
		unixNano,
		2*time.Second,
	)

	// ignore interval when it doesn't exceed the time difference
	check(" AS OF SYSTEM TIME '"+unixNanoStr+"' ",
		unixNano+time.Second.Nanoseconds(),
		unixNano,
		-time.Second,
	)

	// limit to interval when the time between now and baseline is large
	check(" AS OF SYSTEM TIME '-1s' ",
		unixNano+time.Minute.Nanoseconds(),
		unixNano,
		-time.Second,
	)

	// ignore now and baseline when either is zero
	check(" AS OF SYSTEM TIME '-1s' ", 0, unixNano, -time.Second)
	check(" AS OF SYSTEM TIME '-1s' ", unixNano, 0, -time.Second)
	check("", unixNano, 0, 0)
	check("", 0, unixNano, 0)
}
