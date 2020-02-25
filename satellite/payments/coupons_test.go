// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCouponIsExpired(t *testing.T) {
	duration := 2
	now := time.Now().UTC()

	testCases := [...]struct {
		created  time.Time
		expected bool
	}{
		{now.AddDate(0, -duration-1, 0), true},
		{now.AddDate(0, -duration-2, 0), true},
		{now.AddDate(0, -duration-3, 0), true},
		{now.AddDate(0, -1, 0), false},
		{now.AddDate(0, 0, 0), false},
		{now.AddDate(0, 1, 0), false},
	}

	for _, tc := range testCases {
		coupon := Coupon{
			Duration: duration,
			Created:  tc.created,
		}

		require.Equal(t, coupon.IsExpired(), tc.expected)
	}
}
