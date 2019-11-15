// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package date contains various date-related utilities
package date

import "time"

// MonthBoundary extract month from the provided date and returns its edges
func MonthBoundary(t time.Time) (time.Time, time.Time) {
	startDate := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	endDate := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, -1, t.Location())
	return startDate, endDate
}

// DayBoundary returns start and end of the provided day
func DayBoundary(t time.Time) (time.Time, time.Time) {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()),
		time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, -1, t.Location())
}
