// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package payouts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePeriodRange(t *testing.T) {
	testCases := [...]struct {
		periodStart string
		periodEnd   string
		periods     []string
	}{
		{"2020-01", "2020-02", []string{"2020-01", "2020-02"}},
		{"2020-01", "2020-01", []string{"2020-01"}},
		{"2019-11", "2020-02", []string{"2019-11", "2019-12", "2020-01", "2020-02"}},
		{"", "2020-02", nil},
		{"2020-01", "", nil},
		{"2020-01-01", "2020-02", nil},
		{"2020-44", "2020-02", nil},
		{"2020-01", "2020-44", nil},
		{"2020-01", "2019-01", nil},
		{"2020-02", "2020-01", nil},
	}

	for _, tc := range testCases {
		periods, err := parsePeriodRange(tc.periodStart, tc.periodEnd)
		require.Equal(t, len(periods), len(tc.periods))
		if periods != nil {
			for i := 0; i < len(periods); i++ {
				require.Equal(t, periods[i], tc.periods[i])
				require.NoError(t, err)
			}
		} else {
			require.Error(t, err)
		}
	}
}
