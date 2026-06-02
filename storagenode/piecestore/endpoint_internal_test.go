// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsCongested(t *testing.T) {
	const congestionThreshold = 0.8

	tests := []struct {
		name                  string
		maxConcurrentRequests int
		liveRequests          int32
		want                  bool
	}{
		// 0 means unlimited, so it must never report congestion — otherwise the
		// slow-upload check (skipped while congested) is silently disabled.
		{name: "unlimited is never congested", maxConcurrentRequests: 0, liveRequests: 1, want: false},
		{name: "unlimited stays uncongested under heavy load", maxConcurrentRequests: 0, liveRequests: 100000, want: false},
		{name: "negative is treated as unlimited", maxConcurrentRequests: -1, liveRequests: 100, want: false},

		// With a cap, congestion is strictly above the threshold (80% of 100).
		{name: "below threshold", maxConcurrentRequests: 100, liveRequests: 50, want: false},
		{name: "at threshold is not congested", maxConcurrentRequests: 100, liveRequests: 80, want: false},
		{name: "above threshold is congested", maxConcurrentRequests: 100, liveRequests: 81, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint := &Endpoint{
				config: Config{
					MaxConcurrentRequests:             tt.maxConcurrentRequests,
					MinUploadSpeedCongestionThreshold: congestionThreshold,
				},
				liveRequests: tt.liveRequests,
			}
			require.Equal(t, tt.want, endpoint.isCongested())
		})
	}
}
