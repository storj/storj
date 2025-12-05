// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/storj/satellite/reputation"
)

func TestAddAuditToHistory(t *testing.T) {
	config := reputation.AuditHistoryConfig{
		WindowSize:               time.Hour,
		TrackingPeriod:           2 * time.Hour,
		GracePeriod:              time.Hour,
		OfflineThreshold:         0.6,
		OfflineDQEnabled:         true,
		OfflineSuspensionEnabled: true,
	}

	startingWindow := time.Now().Truncate(time.Hour)
	windowsInTrackingPeriod := int(config.TrackingPeriod.Seconds() / config.WindowSize.Seconds())
	currentWindow := startingWindow

	history := &pb.AuditHistory{}

	// online score should be 1 until the first window is finished
	err := reputation.AddAuditToHistory(history, false, currentWindow.Add(2*time.Minute), config)
	require.NoError(t, err)
	require.EqualValues(t, 1, history.Score)

	err = reputation.AddAuditToHistory(history, true, currentWindow.Add(20*time.Minute), config)
	require.NoError(t, err)
	require.EqualValues(t, 1, history.Score)

	// move to next window
	currentWindow = currentWindow.Add(time.Hour)

	// online score should be now be 0.5 since the first window is complete with one online audit and one offline audit
	err = reputation.AddAuditToHistory(history, false, currentWindow.Add(2*time.Minute), config)
	require.NoError(t, err)
	require.EqualValues(t, 0.5, history.Score)

	err = reputation.AddAuditToHistory(history, true, currentWindow.Add(20*time.Minute), config)
	require.NoError(t, err)
	require.EqualValues(t, 0.5, history.Score)

	// move to next window
	currentWindow = currentWindow.Add(time.Hour)

	// try to add an audit for an old window, expect error
	err = reputation.AddAuditToHistory(history, true, startingWindow, config)
	require.Error(t, err)

	// add another online audit for the latest window; score should still be 0.5
	err = reputation.AddAuditToHistory(history, true, currentWindow, config)
	require.NoError(t, err)
	require.EqualValues(t, 0.5, history.Score)
	// add another online audit for the latest window; score should still be 0.5
	err = reputation.AddAuditToHistory(history, true, currentWindow.Add(45*time.Minute), config)
	require.NoError(t, err)
	require.EqualValues(t, 0.5, history.Score)

	currentWindow = currentWindow.Add(time.Hour)
	// in the current state, there are windowsInTrackingPeriod windows with a score of 0.5
	// and one window with a score of 1.0. The Math below calculates the new score when the latest
	// window gets included in the tracking period, and the earliest 0.5 window gets dropped.
	expectedScore := (0.5*float64(windowsInTrackingPeriod-1) + 1) / float64(windowsInTrackingPeriod)
	// add online audit for next window; score should now be expectedScore
	err = reputation.AddAuditToHistory(history, true, currentWindow.Add(time.Minute), config)
	require.NoError(t, err)
	require.EqualValues(t, expectedScore, history.Score)
}

func TestMergeAuditHistoriesWithSingleAudit(t *testing.T) {
	config := reputation.AuditHistoryConfig{
		WindowSize:               time.Hour,
		TrackingPeriod:           2 * time.Hour,
		GracePeriod:              time.Hour,
		OfflineThreshold:         0.6,
		OfflineDQEnabled:         true,
		OfflineSuspensionEnabled: true,
	}

	startingWindow := time.Now().Truncate(time.Hour)
	windowsInTrackingPeriod := int(config.TrackingPeriod.Seconds() / config.WindowSize.Seconds())
	currentWindow := startingWindow

	history := &pb.AuditHistory{}

	// online score should be 1 until the first window is finished
	trackingPeriodFull := testMergeAuditHistories(history, false, currentWindow.Add(2*time.Minute), config)
	require.EqualValues(t, 1, history.Score)
	require.False(t, trackingPeriodFull)

	trackingPeriodFull = testMergeAuditHistories(history, true, currentWindow.Add(20*time.Minute), config)
	require.EqualValues(t, 1, history.Score)
	require.False(t, trackingPeriodFull)

	// move to next window
	currentWindow = currentWindow.Add(time.Hour)

	// online score should be now be 0.5 since the first window is complete with one online audit and one offline audit
	trackingPeriodFull = testMergeAuditHistories(history, false, currentWindow.Add(2*time.Minute), config)
	require.EqualValues(t, 0.5, history.Score)
	require.False(t, trackingPeriodFull)

	trackingPeriodFull = testMergeAuditHistories(history, true, currentWindow.Add(20*time.Minute), config)
	require.EqualValues(t, 0.5, history.Score)
	require.False(t, trackingPeriodFull)

	// move to next window
	currentWindow = currentWindow.Add(time.Hour)

	// add another online audit for the latest window; score should still be 0.5
	trackingPeriodFull = testMergeAuditHistories(history, true, currentWindow, config)
	require.EqualValues(t, 0.5, history.Score)
	// now that we have two full windows other than the current one, tracking period should be considered full.
	require.True(t, trackingPeriodFull)
	// add another online audit for the latest window; score should still be 0.5
	trackingPeriodFull = testMergeAuditHistories(history, true, currentWindow.Add(45*time.Minute), config)
	require.EqualValues(t, 0.5, history.Score)
	require.True(t, trackingPeriodFull)

	currentWindow = currentWindow.Add(time.Hour)
	// in the current state, there are windowsInTrackingPeriod windows with a score of 0.5
	// and one window with a score of 1.0. The Math below calculates the new score when the latest
	// window gets included in the tracking period, and the earliest 0.5 window gets dropped.
	expectedScore := (0.5*float64(windowsInTrackingPeriod-1) + 1) / float64(windowsInTrackingPeriod)
	// add online audit for next window; score should now be expectedScore
	trackingPeriodFull = testMergeAuditHistories(history, true, currentWindow.Add(time.Minute), config)
	require.EqualValues(t, expectedScore, history.Score)
	require.True(t, trackingPeriodFull)
}

func testMergeAuditHistories(history *pb.AuditHistory, online bool, auditTime time.Time, config reputation.AuditHistoryConfig) bool {
	onlineCount := int32(0)
	if online {
		onlineCount = 1
	}
	windows := []*pb.AuditWindow{{
		WindowStart: auditTime.Truncate(config.WindowSize),
		OnlineCount: onlineCount,
		TotalCount:  1,
	}}
	return reputation.MergeAuditHistories(history, windows, config)
}

type hist struct {
	online  bool
	startAt time.Time
}

func TestMergeAuditHistoriesWithMultipleAudits(t *testing.T) {
	t.Parallel()

	config := reputation.AuditHistoryConfig{
		WindowSize:     10 * time.Minute,
		TrackingPeriod: 1 * time.Hour,
	}
	startTime := time.Now().Truncate(time.Hour).Add(-time.Hour)

	t.Run("normal-merge", func(t *testing.T) {
		history := makeHistory([]hist{
			// first window: half online
			{true, startTime},
			{false, startTime.Add(1 * time.Minute)},
			{true, startTime.Add(5 * time.Minute)},
			{false, startTime.Add(8 * time.Minute)},
			// second window: all online
			{true, startTime.Add(10 * time.Minute)},
			{true, startTime.Add(11 * time.Minute)},
			{true, startTime.Add(20*time.Minute - time.Second)},
			// third window: all online
			{true, startTime.Add(20 * time.Minute)},
			// fourth window: all online
			{true, startTime.Add(30 * time.Minute)},
			// fifth window; won't be included in score
			{false, startTime.Add(40 * time.Minute)},
		}, config)
		require.Equal(t, float64(0.875), history.Score) // 3.5/4; chosen to be exact in floating point

		// make the second, third, and fourth windows go from all-online to half-online
		addHistory := makeHistory([]hist{
			// fits in second window
			{false, startTime.Add(12 * time.Minute)},
			{false, startTime.Add(13 * time.Minute)},
			{false, startTime.Add(14 * time.Minute)},
			// fits in third window
			{false, startTime.Add(20*time.Minute + time.Microsecond)},
			// fits in fourth window
			{false, startTime.Add(40*time.Minute - time.Microsecond)},
		}, config)
		require.Equal(t, float64(0), addHistory.Score)

		periodFull := reputation.MergeAuditHistories(history, addHistory.Windows, config)

		require.False(t, periodFull)
		require.Equal(t, 5, len(history.Windows))
		require.Equal(t, float64(0.5), history.Score) // all windows at 50% online
	})

	t.Run("trim-old-windows", func(t *testing.T) {
		history := makeHistory([]hist{
			// this window is too old
			{true, startTime.Add(-2 * time.Minute)},
			{true, startTime.Add(-1 * time.Minute)},
			// oldest window
			{false, startTime.Add(0)},
			// newest window (not included in score)
			{true, startTime.Add(1 * time.Hour)},
		}, config)
		require.Equal(t, float64(0.5), history.Score) // the too-old window is still included in the score here

		addHistory := makeHistory([]hist{
			// this window is too old
			{true, startTime.Add(-10 * time.Minute)},
			// oldest window
			{false, startTime.Add(9 * time.Minute)},
			// a window entirely not present in the other history
			{true, startTime.Add(10 * time.Minute)},
		}, config)
		require.Equal(t, float64(0.5), addHistory.Score) // the latest window is not included (yet)

		periodFull := reputation.MergeAuditHistories(history, addHistory.Windows, config)

		require.False(t, periodFull)
		require.Equal(t, 3, len(history.Windows))
		// oldest window = 0/2, second window = 1/1, third window not counted
		require.Equal(t, float64(0.5), history.Score)
	})

	t.Run("merge-with-empty", func(t *testing.T) {
		history := makeHistory([]hist{}, config)
		require.Equal(t, float64(1), history.Score)

		addHistory := makeHistory([]hist{
			{true, startTime.Add(0)},
			{false, startTime.Add(10 * time.Minute)},
			{false, startTime.Add(59 * time.Minute)},
		}, config)
		require.Equal(t, float64(0.5), addHistory.Score)

		periodFull := reputation.MergeAuditHistories(history, addHistory.Windows, config)

		require.False(t, periodFull)
		require.Equal(t, 3, len(history.Windows))
		require.Equal(t, float64(0.5), history.Score)

		// now merge with an empty addHistory instead
		addHistory = makeHistory([]hist{}, config)
		require.Equal(t, float64(1), addHistory.Score)

		periodFull = reputation.MergeAuditHistories(history, addHistory.Windows, config)

		require.False(t, periodFull)
		require.Equal(t, 3, len(history.Windows))
		require.Equal(t, float64(0.5), history.Score)

		// and finally, merge two empty histories with each other
		history = makeHistory([]hist{}, config)
		addHistory = makeHistory([]hist{}, config)

		periodFull = reputation.MergeAuditHistories(history, addHistory.Windows, config)

		require.False(t, periodFull)
		require.Equal(t, 0, len(history.Windows))
		require.Equal(t, float64(1), history.Score)
	})
}

func makeHistory(histWindows []hist, config reputation.AuditHistoryConfig) *pb.AuditHistory {
	windows := make([]*pb.AuditWindow, 0, len(histWindows))
	for _, histWindow := range histWindows {
		onlineCount := int32(0)
		if histWindow.online {
			onlineCount = 1
		}
		startAt := histWindow.startAt.Truncate(config.WindowSize)
		if len(windows) > 0 && startAt == windows[len(windows)-1].WindowStart {
			windows[len(windows)-1].OnlineCount += onlineCount
			windows[len(windows)-1].TotalCount++
		} else {
			windows = append(windows, &pb.AuditWindow{
				OnlineCount: onlineCount,
				TotalCount:  1,
				WindowStart: startAt,
			})
		}
	}
	baseHistory := &pb.AuditHistory{
		Windows: windows,
	}
	reputation.RecalculateScore(baseHistory)
	return baseHistory
}
