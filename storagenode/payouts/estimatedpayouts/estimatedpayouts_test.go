// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package estimatedpayouts_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/payouts/estimatedpayouts"
)

func TestCurrentMonthExpectations(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		StorageNodeCount: 1,
		SatelliteCount:   2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const payout = 100.0

		type test struct {
			time     time.Time
			expected float64
		}
		tests := []test{
			// 28 days in month
			{time.Date(2021, 2, 1, 16, 0, 0, 0, time.UTC), 2800.00},
			{time.Date(2021, 2, 28, 10, 0, 0, 0, time.UTC), 103.70},
			// 31 days in month
			{time.Date(2021, 3, 1, 19, 0, 0, 0, time.UTC), 3100.0},
			{time.Date(2021, 3, 31, 21, 0, 0, 0, time.UTC), 103.33},
		}

		for _, test := range tests {
			estimates := estimatedpayouts.EstimatedPayout{
				CurrentMonth: estimatedpayouts.PayoutMonthly{
					Payout: payout,
				},
			}

			estimates.SetExpectedMonth(test.time)
			require.False(t, math.IsNaN(estimates.CurrentMonthExpectations))
			require.InDelta(t, test.expected, estimates.CurrentMonthExpectations, 0.01)
		}
	})
}
