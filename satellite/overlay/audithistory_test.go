// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
)

func TestAuditHistoryBasic(t *testing.T) {
	var auditHistoryConfig overlay.AuditHistoryConfig
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.AuditHistory.WindowSize = time.Hour
				config.Overlay.AuditHistory.TrackingPeriod = 2 * time.Hour
				auditHistoryConfig = config.Overlay.AuditHistory
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		cache := planet.Satellites[0].Overlay.Service

		startingWindow := time.Now().Truncate(time.Hour)
		windowsInTrackingPeriod := int(auditHistoryConfig.TrackingPeriod.Seconds() / auditHistoryConfig.WindowSize.Seconds())
		currentWindow := startingWindow

		// online score should be 1 until the first window is finished
		res, err := cache.UpdateAuditHistory(ctx, node.ID(), currentWindow.Add(2*time.Minute), false)
		require.NoError(t, err)
		require.EqualValues(t, 1, res.NewScore)
		require.False(t, res.TrackingPeriodFull)

		res, err = cache.UpdateAuditHistory(ctx, node.ID(), currentWindow.Add(20*time.Minute), true)
		require.NoError(t, err)
		require.EqualValues(t, 1, res.NewScore)
		require.False(t, res.TrackingPeriodFull)

		// move to next window
		currentWindow = currentWindow.Add(time.Hour)

		// online score should be now be 0.5 since the first window is complete with one online audit and one offline audit
		res, err = cache.UpdateAuditHistory(ctx, node.ID(), currentWindow.Add(2*time.Minute), false)
		require.NoError(t, err)
		require.EqualValues(t, 0.5, res.NewScore)
		require.False(t, res.TrackingPeriodFull)

		res, err = cache.UpdateAuditHistory(ctx, node.ID(), currentWindow.Add(20*time.Minute), true)
		require.NoError(t, err)
		require.EqualValues(t, 0.5, res.NewScore)
		require.False(t, res.TrackingPeriodFull)

		// move to next window
		currentWindow = currentWindow.Add(time.Hour)

		// try to add an audit for an old window, expect error
		_, err = cache.UpdateAuditHistory(ctx, node.ID(), startingWindow, true)
		require.Error(t, err)

		// add another online audit for the latest window; score should still be 0.5
		res, err = cache.UpdateAuditHistory(ctx, node.ID(), currentWindow, true)
		require.NoError(t, err)
		require.EqualValues(t, 0.5, res.NewScore)
		// now that we have two full windows other than the current one, tracking period should be considered full.
		require.True(t, res.TrackingPeriodFull)
		// add another online audit for the latest window; score should still be 0.5
		res, err = cache.UpdateAuditHistory(ctx, node.ID(), currentWindow.Add(45*time.Minute), true)
		require.NoError(t, err)
		require.EqualValues(t, 0.5, res.NewScore)
		require.True(t, res.TrackingPeriodFull)

		currentWindow = currentWindow.Add(time.Hour)
		// in the current state, there are windowsInTrackingPeriod windows with a score of 0.5
		// and one window with a score of 1.0. The Math below calculates the new score when the latest
		// window gets included in the tracking period, and the earliest 0.5 window gets dropped.
		expectedScore := (0.5*float64(windowsInTrackingPeriod-1) + 1) / float64(windowsInTrackingPeriod)
		// add online audit for next window; score should now be expectedScore
		res, err = cache.UpdateAuditHistory(ctx, node.ID(), currentWindow.Add(time.Minute), true)
		require.NoError(t, err)
		require.EqualValues(t, expectedScore, res.NewScore)
		require.True(t, res.TrackingPeriodFull)
	})
}
