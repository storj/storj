// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package date contains various date-related utilities
package date

import "time"

// MonthBoundary return first and last day of current month
func MonthBoundary() (firstDay, lastDay time.Time) {
	now := time.Now()
	currentYear, currentMonth, _ := now.Date()
	currentLocation := now.Location()

	firstDay = time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, currentLocation)
	lastDay = firstDay.AddDate(0, 1, -1)

	return
}

// DayBoundary returns start and end of the provided day
func DayBoundary(t time.Time) (time.Time, time.Time) {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC),
		time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, -1, time.UTC)
}
