// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectStorageThresholds(t *testing.T) {
	const limit = 1000
	// storageEnabled is the feature-enable bit that opts a project in to storage notifications.
	storageEnabled := int(StorageNotificationsEnabled)

	t.Run("no events when below all thresholds", func(t *testing.T) {
		thresholds, resets := detectStorageThresholds(0, 100, limit, storageEnabled)
		require.Empty(t, thresholds)
		require.Empty(t, resets)
	})

	t.Run("80% threshold crossed upward", func(t *testing.T) {
		thresholds, resets := detectStorageThresholds(799, 801, limit, storageEnabled)
		require.Equal(t, []ProjectUsageThreshold{StorageUsage80}, thresholds)
		require.Empty(t, resets)
	})

	t.Run("exactly at 80% boundary counts as crossed", func(t *testing.T) {
		// before=799, after=800 → exactly at threshold.
		thresholds, resets := detectStorageThresholds(799, 800, limit, storageEnabled)
		require.Equal(t, []ProjectUsageThreshold{StorageUsage80}, thresholds)
		require.Empty(t, resets)
	})

	t.Run("100% threshold crossed upward", func(t *testing.T) {
		thresholds, resets := detectStorageThresholds(999, 1000, limit, storageEnabled)
		require.Equal(t, []ProjectUsageThreshold{StorageUsage100}, thresholds)
		require.Empty(t, resets)
	})

	t.Run("only 100% event when both thresholds crossed in single upload", func(t *testing.T) {
		thresholds, resets := detectStorageThresholds(0, 1000, limit, storageEnabled)
		require.Equal(t, []ProjectUsageThreshold{StorageUsage100}, thresholds)
		require.Empty(t, resets)
	})

	t.Run("no event when 80% flag already set", func(t *testing.T) {
		thresholds, resets := detectStorageThresholds(799, 800, limit, storageEnabled|int(StorageUsage80))
		require.Empty(t, thresholds)
		require.Empty(t, resets)
	})

	t.Run("no event when already above threshold before upload", func(t *testing.T) {
		thresholds, resets := detectStorageThresholds(800, 900, limit, storageEnabled)
		require.Empty(t, thresholds)
		require.Empty(t, resets)
	})

	t.Run("80% reset when flag set and usage drops below threshold", func(t *testing.T) {
		thresholds, resets := detectStorageThresholds(0, 700, limit, storageEnabled|int(StorageUsage80))
		require.Empty(t, thresholds)
		require.Equal(t, []ProjectUsageThreshold{StorageUsage80}, resets)
	})

	t.Run("no reset when usage drops below threshold but flag not set", func(t *testing.T) {
		thresholds, resets := detectStorageThresholds(800, 700, limit, storageEnabled)
		require.Empty(t, thresholds)
		require.Empty(t, resets)
	})

	t.Run("only 100% event when crossing 100% even though 80% flag not set", func(t *testing.T) {
		// Verifies that only the highest newly-crossed threshold is emitted:
		// the 80% event is suppressed because 100% was also crossed in the same upload.
		thresholds, resets := detectStorageThresholds(900, 1000, limit, storageEnabled)
		require.Equal(t, []ProjectUsageThreshold{StorageUsage100}, thresholds)
		require.Empty(t, resets)
	})

	t.Run("both reset events when both flags set and usage at zero", func(t *testing.T) {
		flags := storageEnabled | int(StorageUsage80) | int(StorageUsage100)
		thresholds, resets := detectStorageThresholds(0, 0, limit, flags)
		require.Empty(t, thresholds)
		require.Equal(t, []ProjectUsageThreshold{StorageUsage100, StorageUsage80}, resets)
	})
}

func TestDetectBandwidthThresholds(t *testing.T) {
	const limit = 1000
	// egressEnabled is the feature-enable bit that opts a project in to egress notifications.
	egressEnabled := int(EgressNotificationsEnabled)

	t.Run("no events when opted out (flags=0)", func(t *testing.T) {
		thresholds, resets := detectBandwidthThresholds(800, limit, 0)
		require.Empty(t, thresholds)
		require.Empty(t, resets)
	})

	t.Run("no events when below all thresholds", func(t *testing.T) {
		thresholds, resets := detectBandwidthThresholds(100, limit, egressEnabled)
		require.Empty(t, thresholds)
		require.Empty(t, resets)
	})

	t.Run("80% event when current at or above threshold and flag not set", func(t *testing.T) {
		thresholds, resets := detectBandwidthThresholds(800, limit, egressEnabled)
		require.Equal(t, []ProjectUsageThreshold{EgressUsage80}, thresholds)
		require.Empty(t, resets)
	})

	t.Run("only 100% event when current at or above 100% threshold and flag not set", func(t *testing.T) {
		thresholds, resets := detectBandwidthThresholds(1000, limit, egressEnabled)
		require.Equal(t, []ProjectUsageThreshold{EgressUsage100}, thresholds)
		require.Empty(t, resets)
	})

	t.Run("no event when flag already set", func(t *testing.T) {
		thresholds, resets := detectBandwidthThresholds(800, limit, egressEnabled|int(EgressUsage80))
		require.Empty(t, thresholds)
		require.Empty(t, resets)
	})

	t.Run("reset when below threshold and flag set", func(t *testing.T) {
		thresholds, resets := detectBandwidthThresholds(0, limit, egressEnabled|int(EgressUsage80))
		require.Empty(t, thresholds)
		require.Equal(t, []ProjectUsageThreshold{EgressUsage80}, resets)
	})

	t.Run("both resets at start of new month with both flags set", func(t *testing.T) {
		flags := egressEnabled | int(EgressUsage80) | int(EgressUsage100)
		thresholds, resets := detectBandwidthThresholds(0, limit, flags)
		require.Empty(t, thresholds)
		require.Equal(t, []ProjectUsageThreshold{EgressUsage100, EgressUsage80}, resets)
	})
}
