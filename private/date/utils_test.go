// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package date_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/private/date"
)

func TestMonthBoundary(t *testing.T) {
	now := time.Now()

	start, end := date.MonthBoundary(now)
	assert.Equal(t, start, time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()))
	assert.Equal(t, end, time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, -1, now.Location()))
}

func TestDayBoundary(t *testing.T) {
	now := time.Now()

	start, end := date.DayBoundary(now)
	assert.Equal(t, start, time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))
	assert.Equal(t, end, time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, -1, now.Location()))
}

func TestPeriodToTime(t *testing.T) {
	testCases := [...]struct {
		period     string
		periodTime time.Time
	}{
		{"2020-01", time.Date(2020, 01, 01, 0, 0, 0, 0, &time.Location{})},
		{"2020-02-01", time.Date(2020, 02, 01, 0, 0, 0, 0, &time.Location{})},
		{"2019-11-04 14:14:14", time.Date(2019, 11, 01, 0, 0, 0, 0, &time.Location{})},
		{"2020-03-04T15:04:05-0700", time.Date(2020, 03, 01, 0, 0, 0, 0, &time.Location{})},
		{"2020-04gasgahsgnasjfasgjs", time.Date(2020, 04, 01, 0, 0, 0, 0, &time.Location{})},
	}

	for _, tc := range testCases {
		periodTime, err := date.PeriodToTime(tc.period)
		require.NoError(t, err)
		require.Equal(t, periodTime.String(), tc.periodTime.String())
	}
}

func TestPeriodToTime_Invalid(t *testing.T) {
	testCases := [...]struct {
		period string
		error  string
	}{
		{"", "invalid period \"\""},
		{"2020", "invalid period \"2020\""},
	}

	for _, tc := range testCases {
		_, err := date.PeriodToTime(tc.period)
		require.ErrorContains(t, err, tc.error)
	}
}

func TestMonthsBetweenDates(t *testing.T) {
	testCases := [...]struct {
		from         time.Time
		to           time.Time
		monthsAmount int
	}{
		{time.Date(2020, 2, 13, 0, 0, 0, 0, &time.Location{}), time.Date(2020, 05, 13, 0, 0, 0, 0, &time.Location{}), 3},
		{time.Date(2015, 7, 30, 0, 0, 0, 0, &time.Location{}), time.Date(2020, 05, 13, 0, 0, 0, 0, &time.Location{}), 58},
		{time.Date(2017, 1, 28, 0, 0, 0, 0, &time.Location{}), time.Date(2020, 05, 13, 0, 0, 0, 0, &time.Location{}), 40},
		{time.Date(2016, 11, 1, 0, 0, 0, 0, &time.Location{}), time.Date(2020, 05, 13, 0, 0, 0, 0, &time.Location{}), 42},
		{time.Date(2019, 4, 17, 0, 0, 0, 0, &time.Location{}), time.Date(2020, 05, 13, 0, 0, 0, 0, &time.Location{}), 13},
		{time.Date(2018, 9, 11, 0, 0, 0, 0, &time.Location{}), time.Date(2020, 05, 13, 0, 0, 0, 0, &time.Location{}), 20},
	}

	for _, tc := range testCases {
		monthDiff := date.MonthsBetweenDates(tc.from, tc.to)
		require.Equal(t, monthDiff, tc.monthsAmount)
	}
}
