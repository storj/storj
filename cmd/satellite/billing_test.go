// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidYearMonth(t *testing.T) {
	yearMonth := "2020-12"
	periodStart, err := parseYearMonth(yearMonth)
	require.NoError(t, err)
	require.Equal(t, 2020, periodStart.Year())
	require.Equal(t, "December", periodStart.Month().String())
	require.Equal(t, 01, periodStart.Day())
	require.Equal(t, "UTC", periodStart.Location().String())
}

func TestInvalidYearMonth(t *testing.T) {
	invalidYearMonth := []string{
		"2020-13",
		"2020-00",
		"123-01",
		"1999-3",
	}

	for _, invalid := range invalidYearMonth {
		date, err := parseYearMonth(invalid)
		require.Equal(t, date, time.Time{})
		require.Error(t, err)
	}
}
