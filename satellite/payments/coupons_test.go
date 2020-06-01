// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCoupon_ExpirationDate(t *testing.T) {
	for _, tt := range []struct {
		created  time.Time
		duration int
		expires  time.Time
	}{
		{
			created:  time.Date(2020, 1, 30, 0, 0, 0, 0, time.UTC), // 2020-01-30 00:00:00 +0000 UTC
			duration: 0,                                            // sign-up month only
			expires:  time.Date(2020, 2, 0, 0, 0, 0, 0, time.UTC),  // 2020-01-31 00:00:00 +0000 UTC
		},
		{
			created:  time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC), // 2020-02-01 00:00:00 +0000 UTC
			duration: 1,                                           // sign-up month + 1 full month
			expires:  time.Date(2020, 4, 0, 0, 0, 0, 0, time.UTC), // 2020-03-31 00:00:00 +0000 UTC
		},
		{
			created:  time.Date(2020, 2, 5, 8, 0, 0, 0, time.UTC), // 2020-02-05 08:00:00 +0000 UTC
			duration: 2,                                           // sign-up month + 2 full months
			expires:  time.Date(2020, 5, 0, 0, 0, 0, 0, time.UTC), // 2020-04-30 00:00:00 +0000 UTC
		},
	} {
		coupon := Coupon{
			Duration: tt.duration,
			Created:  tt.created,
		}
		require.Equal(t, tt.expires, coupon.ExpirationDate())
	}
}
