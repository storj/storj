package reputation_test

import (
	"testing"
	"time"

	"storj.io/common/pb"
	"storj.io/storj/satellite/reputation"
)

func Test_CheatAddAuditToHistory(t *testing.T) {
	config := reputation.AuditHistoryConfig{
		WindowSize:               time.Hour,
		TrackingPeriod:           2 * time.Hour,
		GracePeriod:              time.Hour,
		OfflineThreshold:         0.6,
		OfflineDQEnabled:         true,
		OfflineSuspensionEnabled: true,
	}
	// Define test cases
	testCases := []struct {
		desc      string
		online    bool
		auditTime time.Time
		config    reputation.AuditHistoryConfig
		expected  error
	}{
		{
			desc:      "Test adding audit to empty history",
			online:    true,
			auditTime: time.Now(),
			config:    config,
			expected:  nil,
		},
		{
			desc:      "Test adding audit to existing window",
			online:    false,
			auditTime: time.Now(),
			config:    reputation.AuditHistoryConfig{WindowSize: time.Hour, TrackingPeriod: time.Hour * 24},
			expected:  nil,
		},
		{
			desc:      "Test adding audit outside of tracking period",
			online:    true,
			auditTime: time.Now().Add(-time.Hour * 48),
			config:    reputation.AuditHistoryConfig{WindowSize: time.Hour, TrackingPeriod: time.Hour * 24},
			expected:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Define the initial audit history
			a := &pb.AuditHistory{}

			// Add audit to history
			err := reputation.AddAuditToHistory(a, tc.online, tc.auditTime, tc.config)

			// Check if the returned error is as expected
			if err != tc.expected {
				t.Fatalf("Expected error to be %v, but got %v", tc.expected, err)
			}
		})
	}
}
