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

// PeriodToTime returns time.Time period in format YYYY-MM from string.
func PeriodToTime(period string) (_ time.Time, err error) {
	layout := "2006-01"
	shortPeriod := period[0:7]
	result, err := time.Parse(layout, shortPeriod)
	if err != nil {
		return time.Time{}, err
	}

	return result, nil
}

// MonthsCountSince calculates the months between now and the createdAtTime time.Time value passed.
func MonthsCountSince(from time.Time) int {
	now := time.Now().UTC()
	return MonthsBetweenDates(from, now)
}

// MonthsBetweenDates calculates amount of months between two dates
func MonthsBetweenDates(from time.Time, to time.Time) int {
	months := 0
	month := from.Month()
	for from.Before(to) {
		from = from.Add(time.Hour * 24)
		nextMonth := from.Month()
		if nextMonth != month {
			months++
		}
		month = nextMonth
	}

	return months
}
