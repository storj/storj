// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package estimatedpayouts_test

import (
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
		estimatedPayout := estimatedpayouts.EstimatedPayout{
			CurrentMonth: estimatedpayouts.PayoutMonthly{
				Payout: 100,
			},
		}

		currentDay := time.Now().Day() - 1
		now := time.Now().UTC()
		y, m, _ := now.Date()
		daysInMonth := time.Date(y, m+1, 1, 0, 0, 0, -1, &time.Location{}).Day()

		expectations := (estimatedPayout.CurrentMonth.Payout / float64(currentDay)) * float64(daysInMonth)
		estimatedPayout.SetExpectedMonth(now)
		require.Equal(t, estimatedPayout.CurrentMonthExpectations, expectations)
	})
}
