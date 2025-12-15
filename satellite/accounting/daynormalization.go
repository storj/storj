// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"time"

	"github.com/zeebo/errs"
)

// DailyUsageEvent represents usage for a single day.
type DailyUsageEvent struct {
	Date   time.Time // Start of the day (midnight UTC)
	Amount float64
}

// NormalizeAcrossDayBoundaries splits a total amount of usage between the days spanned by
// startTime and endTime. Each day's amount is proportional to the duration of usage within that day.
func NormalizeAcrossDayBoundaries(startTime, endTime time.Time, totalAmount float64) ([]DailyUsageEvent, error) {
	startTime = startTime.UTC()
	endTime = endTime.UTC()

	if endTime.Before(startTime) {
		return nil, errs.New("invalid time range: endTime is before startTime")
	}

	totalDuration := endTime.Sub(startTime)
	if totalDuration == 0 {
		return []DailyUsageEvent{
			{
				Date:   startTime.UTC().Truncate(24 * time.Hour),
				Amount: totalAmount,
			},
		}, nil
	}

	var events []DailyUsageEvent

	currentDayStart := startTime.UTC().Truncate(24 * time.Hour)
	nextDayStart := currentDayStart.Add(24 * time.Hour)

	currentTime := startTime

	for currentTime.Before(endTime) {
		currentDayEnd := nextDayStart
		if endTime.Before(nextDayStart) {
			currentDayEnd = endTime
		}

		dayDuration := currentDayEnd.Sub(currentTime)

		// Calculate proportional amount for this day
		proportion := float64(dayDuration) / float64(totalDuration)
		dayAmount := totalAmount * proportion

		events = append(events, DailyUsageEvent{
			Date:   currentDayStart,
			Amount: dayAmount,
		})

		// Move to next day
		currentTime = nextDayStart
		currentDayStart = nextDayStart
		nextDayStart = nextDayStart.Add(24 * time.Hour)
	}

	return events, nil
}
