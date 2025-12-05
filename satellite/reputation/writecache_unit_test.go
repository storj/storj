// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
)

func TestNextTimeForSync(t *testing.T) {
	var zeroesID storj.NodeID
	binary.BigEndian.PutUint64(zeroesID[:8], 0) // unnecessary, but for clarity

	var halfwayID storj.NodeID
	binary.BigEndian.PutUint64(halfwayID[:8], 1<<63)

	var quarterwayID storj.NodeID
	binary.BigEndian.PutUint64(quarterwayID[:8], 1<<62)

	const (
		zeroOffset       = 0
		halfwayOffset    = 1 << 63
		quarterwayOffset = 1 << 62
	)

	startOfHour := time.Now().Truncate(time.Hour)
	now := startOfHour.Add(15 * time.Minute)

	nextTime := nextTimeForSync(zeroOffset, halfwayID, now, time.Hour)
	requireInDeltaTime(t, startOfHour.Add(30*time.Minute), nextTime, time.Second)

	nextTime = nextTimeForSync(halfwayOffset, zeroesID, now, time.Hour)
	requireInDeltaTime(t, startOfHour.Add(30*time.Minute), nextTime, time.Second)

	nextTime = nextTimeForSync(zeroOffset, zeroesID, now, time.Hour)
	requireInDeltaTime(t, startOfHour.Add(time.Hour), nextTime, time.Second)

	nextTime = nextTimeForSync(halfwayOffset, halfwayID, now, time.Hour)
	requireInDeltaTime(t, startOfHour.Add(time.Hour), nextTime, time.Second)

	nextTime = nextTimeForSync(quarterwayOffset, halfwayID, now, time.Hour)
	requireInDeltaTime(t, startOfHour.Add(45*time.Minute), nextTime, time.Second)
}

func requireInDeltaTime(t *testing.T, expected time.Time, actual time.Time, delta time.Duration) {
	if delta < 0 {
		delta = -delta
	}
	require.Falsef(t, actual.Before(expected.Add(-delta)), "%s is not within %s of %s", actual, delta, expected)
	require.Falsef(t, actual.After(expected.Add(delta)), "%s is not within %s of %s", actual, delta, expected)
}
