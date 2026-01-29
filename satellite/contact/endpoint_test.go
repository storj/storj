// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	ekpb "storj.io/eventkit/pb"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/shared/modular/eventkit/eventkitspy"
)

func TestEmitEventkitEvent(t *testing.T) {
	ctx := testcontext.New(t)
	emitEventkitEvent(ctx, &pb.CheckInRequest{
		Address: "127.0.0.1:234",
	}, false, false, overlay.NodeCheckInInfo{}, overlay.CheckInResult{})
}

func TestEmitEventkitEventDowntimeTag(t *testing.T) {
	ctx := testcontext.New(t)

	// Clear any previous events
	eventkitspy.Clear()

	nodeID := testrand.NodeID()
	nodeInfo := overlay.NodeCheckInInfo{
		NodeID: nodeID,
	}

	// Test 1: Regular check-in without CameBackOnline should NOT have downtime-hours tag
	emitEventkitEvent(ctx, &pb.CheckInRequest{
		Address: "127.0.0.1:234",
	}, true, true, nodeInfo, overlay.CheckInResult{
		CameBackOnline: false,
	})

	events := eventkitspy.GetEvents()
	require.NotEmpty(t, events, "Expected at least one event")

	// Find the checkin event
	var lastEvent = events[len(events)-1]
	require.Equal(t, "checkin", lastEvent.Name)

	// Verify downtime-hours tag is NOT present
	hasDowntimeTag := false
	for _, tag := range lastEvent.Tags {
		if tag.Key == "downtime-hours" {
			hasDowntimeTag = true
			break
		}
	}
	require.False(t, hasDowntimeTag, "downtime-hours tag should NOT be present for regular check-in")

	// Test 2: Check-in with CameBackOnline=true SHOULD have downtime-hours tag
	eventkitspy.Clear()

	// Simulate 5 hours and 30 minutes of downtime
	downtime := 5*time.Hour + 30*time.Minute
	emitEventkitEvent(ctx, &pb.CheckInRequest{
		Address: "127.0.0.1:234",
	}, true, true, nodeInfo, overlay.CheckInResult{
		CameBackOnline: true,
		Downtime:       downtime,
	})

	events = eventkitspy.GetEvents()
	require.NotEmpty(t, events, "Expected at least one event")

	lastEvent = events[len(events)-1]
	require.Equal(t, "checkin", lastEvent.Name)

	// Verify downtime-hours tag IS present with correct rounded-up value
	hasDowntimeTag = false
	var downtimeValue int64
	for _, tag := range lastEvent.Tags {
		if tag.Key == "downtime-hours" {
			hasDowntimeTag = true
			if intVal, ok := tag.Value.(*ekpb.Tag_Int64); ok {
				downtimeValue = intVal.Int64
			}
			break
		}
	}
	require.True(t, hasDowntimeTag, "downtime-hours tag should be present when node came back online")
	expectedHours := int64(math.Ceil(downtime.Hours())) // 5.5 hours rounds up to 6
	require.Equal(t, expectedHours, downtimeValue, "downtime-hours should be rounded up to nearest hour")

	// Test 3: Verify exact boundary cases for rounding
	testCases := []struct {
		downtime      time.Duration
		expectedHours int64
	}{
		{4 * time.Hour, 4},                    // exactly 4 hours -> 4
		{4*time.Hour + time.Second, 5},        // 4h 0m 1s -> 5
		{4*time.Hour + 30*time.Minute, 5},     // 4h 30m -> 5
		{5*time.Hour + 59*time.Minute, 6},     // 5h 59m -> 6
		{24 * time.Hour, 24},                  // exactly 24 hours -> 24
		{24*time.Hour + time.Second, 25},      // 24h 1s -> 25
		{24*time.Hour + 59*time.Minute, 25},   // 24h 59m -> 25
		{100*time.Hour + 30*time.Minute, 101}, // 100h 30m -> 101
		{time.Duration(0), 0},                 // 0 hours -> 0
		{30 * time.Minute, 1},                 // 30m -> 1
		{time.Nanosecond, 1},                  // 1ns -> 1 (any non-zero rounds up)
	}

	for _, tc := range testCases {
		eventkitspy.Clear()
		emitEventkitEvent(ctx, &pb.CheckInRequest{
			Address: "127.0.0.1:234",
		}, true, true, nodeInfo, overlay.CheckInResult{
			CameBackOnline: true,
			Downtime:       tc.downtime,
		})

		events = eventkitspy.GetEvents()
		require.NotEmpty(t, events)
		lastEvent = events[len(events)-1]

		for _, tag := range lastEvent.Tags {
			if tag.Key == "downtime-hours" {
				if intVal, ok := tag.Value.(*ekpb.Tag_Int64); ok {
					require.Equal(t, tc.expectedHours, intVal.Int64,
						"downtime %v should round to %d hours", tc.downtime, tc.expectedHours)
				}
				break
			}
		}
	}
}
