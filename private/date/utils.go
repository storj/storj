// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package date contains various date-related utilities
package date

import (
	"fmt"
	"time"
)

// MonthBoundary extract month from the provided date and returns its edges.
func MonthBoundary(t time.Time) (time.Time, time.Time) {
	startDate := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	endDate := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, -1, t.Location())
	return startDate, endDate
}

// DayBoundary returns start and end of the provided day.
func DayBoundary(t time.Time) (time.Time, time.Time) {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()),
		time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, -1, t.Location())
}

// PeriodToTime returns time.Time period in format YYYY-MM from string.
func PeriodToTime(period string) (_ time.Time, err error) {
	if len(period) < 7 {
		return time.Time{}, fmt.Errorf("invalid period %q", period)
	}
	shortPeriod := period[:7]
	layout := "2006-01"
	result, err := time.Parse(layout, shortPeriod)
	if err != nil {
		return time.Time{}, err
	}

	return result, nil
}

// MonthsCountSince calculates the months between now and the createdAtTime time.Time value passed.
func MonthsCountSince(from time.Time) int {
	return MonthsBetweenDates(from, time.Now())
}

// MonthsBetweenDates calculates amount of months between two dates.
func MonthsBetweenDates(from time.Time, to time.Time) int {
	// we need UTC here before its the only sensible way to say what day it is
	y1, M1, _ := from.UTC().Date()
	y2, M2, _ := to.UTC().Date()

	months := ((y2 - y1) * 12) + int(M2) - int(M1)
	// note that according to the tests, we ignore days of the month
	return months
}

// TruncateToHourInNano returns the time truncated to the hour in nanoseconds.
func TruncateToHourInNano(t time.Time) int64 {
	return t.Truncate(1 * time.Hour).UnixNano()
}

// UTCEndOfMonth returns utc end of month (f.e. to get last day in month).
func UTCEndOfMonth(now time.Time) time.Time {
	now = now.UTC()
	y, m, _ := now.Date()
	return time.Date(y, m+1, 1, 0, 0, 0, 0, &time.Location{}).Add(-time.Nanosecond)
}

// UTCBeginOfMonth returns utc begin of month (f.e. to get first day in month at 00h00m).
func UTCBeginOfMonth(now time.Time) time.Time {
	now = now.UTC()
	y, m, _ := now.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, &time.Location{})
}
