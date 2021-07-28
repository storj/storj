// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/internalpb"
)

func TestAuditHistoryBasic(t *testing.T) {
	const windowSize = time.Hour
	const trackingPeriod = 2 * time.Hour

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Reputation.AuditHistory.WindowSize = windowSize
				config.Reputation.AuditHistory.TrackingPeriod = trackingPeriod
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		db := planet.Satellites[0].DB.Reputation()

		startingWindow := time.Now().Truncate(time.Hour)
		windowsInTrackingPeriod := int(trackingPeriod.Seconds() / windowSize.Seconds())
		currentWindow := startingWindow

		config := planet.Satellites[0].Config.Reputation.AuditHistory
		newHistory := &internalpb.AuditHistory{}
		historyBytes, err := pb.Marshal(newHistory)
		require.NoError(t, err)
		// online score should be 1 until the first window is finished
		res, err := db.UpdateAuditHistory(ctx, historyBytes, currentWindow.Add(2*time.Minute), false, config)
		require.NoError(t, err)
		require.EqualValues(t, 1, res.NewScore)
		require.False(t, res.TrackingPeriodFull)

		res, err = db.UpdateAuditHistory(ctx, res.History, currentWindow.Add(20*time.Minute), true, config)
		require.NoError(t, err)
		require.EqualValues(t, 1, res.NewScore)
		require.False(t, res.TrackingPeriodFull)

		// move to next window
		currentWindow = currentWindow.Add(time.Hour)

		// online score should be now be 0.5 since the first window is complete with one online audit and one offline audit
		res, err = db.UpdateAuditHistory(ctx, res.History, currentWindow.Add(2*time.Minute), false, config)
		require.NoError(t, err)
		require.EqualValues(t, 0.5, res.NewScore)
		require.False(t, res.TrackingPeriodFull)

		res, err = db.UpdateAuditHistory(ctx, res.History, currentWindow.Add(20*time.Minute), true, config)
		require.NoError(t, err)
		require.EqualValues(t, 0.5, res.NewScore)
		require.False(t, res.TrackingPeriodFull)

		// move to next window
		currentWindow = currentWindow.Add(time.Hour)

		// try to add an audit for an old window, expect error
		_, err = db.UpdateAuditHistory(ctx, res.History, startingWindow, true, config)
		require.Error(t, err)

		// add another online audit for the latest window; score should still be 0.5
		res, err = db.UpdateAuditHistory(ctx, res.History, currentWindow, true, config)
		require.NoError(t, err)
		require.EqualValues(t, 0.5, res.NewScore)
		// now that we have two full windows other than the current one, tracking period should be considered full.
		require.True(t, res.TrackingPeriodFull)
		// add another online audit for the latest window; score should still be 0.5
		res, err = db.UpdateAuditHistory(ctx, res.History, currentWindow.Add(45*time.Minute), true, config)
		require.NoError(t, err)
		require.EqualValues(t, 0.5, res.NewScore)
		require.True(t, res.TrackingPeriodFull)

		currentWindow = currentWindow.Add(time.Hour)
		// in the current state, there are windowsInTrackingPeriod windows with a score of 0.5
		// and one window with a score of 1.0. The Math below calculates the new score when the latest
		// window gets included in the tracking period, and the earliest 0.5 window gets dropped.
		expectedScore := (0.5*float64(windowsInTrackingPeriod-1) + 1) / float64(windowsInTrackingPeriod)
		// add online audit for next window; score should now be expectedScore
		res, err = db.UpdateAuditHistory(ctx, res.History, currentWindow.Add(time.Minute), true, config)
		require.NoError(t, err)
		require.EqualValues(t, expectedScore, res.NewScore)
		require.True(t, res.TrackingPeriodFull)
	})
}
