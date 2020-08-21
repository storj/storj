// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestAuditHistoryBasic(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.AuditHistory.WindowSize = time.Hour
				config.Overlay.AuditHistory.TrackingPeriod = 2 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		cache := planet.Satellites[0].DB.OverlayCache()
		auditHistoryConfig := planet.Satellites[0].Config.Overlay.AuditHistory

		startingWindow := time.Now().Truncate(time.Hour)
		windowsInTrackingPeriod := int(auditHistoryConfig.TrackingPeriod.Seconds() / auditHistoryConfig.WindowSize.Seconds())
		currentWindow := startingWindow
		// we need windowsInTrackingPeriod+1 windows before we will see scores besides "1"
		// add enough windows to fill the tracking period, each with 1 online and 1 offline audit
		for i := 0; i < windowsInTrackingPeriod; i++ {
			score, err := cache.UpdateAuditHistory(ctx, node.ID(), currentWindow.Add(2*time.Minute), false, auditHistoryConfig)
			require.NoError(t, err)
			require.EqualValues(t, 1, score)

			score, err = cache.UpdateAuditHistory(ctx, node.ID(), currentWindow.Add(20*time.Minute), true, auditHistoryConfig)
			require.NoError(t, err)
			require.EqualValues(t, 1, score)

			// move to next window
			currentWindow = currentWindow.Add(time.Hour)
		}

		// try to add an audit for an old window, expect error
		_, err := cache.UpdateAuditHistory(ctx, node.ID(), startingWindow, true, auditHistoryConfig)
		require.Error(t, err)

		// Add online audit for next window. Score should now be 0.5, since we have a tracking period full
		// of completed windows, each with 50% online audits.
		score, err := cache.UpdateAuditHistory(ctx, node.ID(), currentWindow, true, auditHistoryConfig)
		require.NoError(t, err)
		require.EqualValues(t, 0.5, score)
		// add another online audit for the latest window; score should still be 0.5
		score, err = cache.UpdateAuditHistory(ctx, node.ID(), currentWindow.Add(45*time.Minute), true, auditHistoryConfig)
		require.NoError(t, err)
		require.EqualValues(t, 0.5, score)

		currentWindow = currentWindow.Add(time.Hour)
		// in the current state, there are windowsInTrackingPeriod windows with a score of 0.5
		// and one window with a score of 1.0. The Math below calculates the new score when the latest
		// window gets included in the tracking period, and the earliest 0.5 window gets dropped.
		expectedScore := (0.5*float64(windowsInTrackingPeriod-1) + 1) / float64(windowsInTrackingPeriod)
		// add online audit for next window; score should now be expectedScore
		score, err = cache.UpdateAuditHistory(ctx, node.ID(), currentWindow.Add(time.Minute), true, auditHistoryConfig)
		require.NoError(t, err)
		require.EqualValues(t, expectedScore, score)
	})
}
