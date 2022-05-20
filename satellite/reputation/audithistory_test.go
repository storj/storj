// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/internalpb"
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

	history := &internalpb.AuditHistory{}

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
