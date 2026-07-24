// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package storageusage

import (
	"math"
	"sort"
	"time"

	"storj.io/common/storj"
)

// NormalizeForDisplay replaces completed gaps with the average storage rate
// represented by the next valid satellite tally. It projects a trailing gap
// from the last valid rate. Calculated rows never add synthetic AtRestTotal.
func NormalizeForDisplay(stamps []Stamp, from, through time.Time) []Stamp {
	if len(stamps) == 0 || through.Before(from) {
		return nil
	}

	sorted := append([]Stamp(nil), stamps...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].IntervalStart.Before(sorted[j].IntervalStart)
	})

	fromDay := utcDay(from)
	throughDay := utcDay(through)
	byDay := make(map[time.Time]Stamp)

	var previousValid *Stamp
	var lastRate float64

	for i := range sorted {
		current := sorted[i]
		if !validSatelliteStamp(current) {
			continue
		}

		rate := current.AtRestTotalBytes
		intervalHours := current.IntervalInHours
		if previousValid != nil {
			intervalHours = current.IntervalEndTime.Sub(previousValid.IntervalEndTime).Hours()
			if intervalHours <= 0 {
				continue
			}
			rate = current.AtRestTotal / intervalHours
		} else if intervalHours <= 0 {
			intervalHours = 24
			rate = current.AtRestTotal / intervalHours
		}
		if rate < 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			continue
		}

		currentDay := utcDay(current.IntervalStart)
		if previousValid != nil {
			for day := utcDay(previousValid.IntervalStart).AddDate(0, 0, 1); day.Before(currentDay); day = day.AddDate(0, 0, 1) {
				if day.Before(fromDay) || day.After(throughDay) {
					continue
				}
				byDay[day] = calculatedStamp(current.SatelliteID, day, rate, 24)
			}
		}

		if !currentDay.Before(fromDay) && !currentDay.After(throughDay) {
			current.AtRestTotalBytes = rate
			current.IntervalInHours = intervalHours
			current.Calculated = false
			byDay[currentDay] = current
		}

		previousValid = &current
		lastRate = rate
	}

	if previousValid != nil {
		for day := utcDay(previousValid.IntervalStart).AddDate(0, 0, 1); !day.After(throughDay); day = day.AddDate(0, 0, 1) {
			if day.Before(fromDay) {
				continue
			}
			if _, exists := byDay[day]; exists {
				continue
			}

			hours := float64(24)
			if day.Equal(throughDay) {
				hours = through.Sub(day).Hours()
				if hours <= 0 {
					hours = 24
				}
			}
			byDay[day] = calculatedStamp(previousValid.SatelliteID, day, lastRate, hours)
		}
	}

	result := make([]Stamp, 0, len(byDay))
	for day := fromDay; !day.After(throughDay); day = day.AddDate(0, 0, 1) {
		if stamp, exists := byDay[day]; exists {
			result = append(result, stamp)
		}
	}
	return result
}

// CombineForDisplay sums normalized per-satellite display rows by UTC day.
func CombineForDisplay(series ...[]Stamp) []StampGroup {
	byDay := make(map[time.Time]StampGroup)
	for _, stamps := range series {
		for _, stamp := range stamps {
			day := utcDay(stamp.IntervalStart)
			group := byDay[day]
			group.AtRestTotal += stamp.AtRestTotal
			group.AtRestTotalBytes += stamp.AtRestTotalBytes
			group.IntervalStart = day
			group.Calculated = group.Calculated || stamp.Calculated
			byDay[day] = group
		}
	}

	days := make([]time.Time, 0, len(byDay))
	for day := range byDay {
		days = append(days, day)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })

	result := make([]StampGroup, 0, len(days))
	for _, day := range days {
		result = append(result, byDay[day])
	}
	return result
}

// DisplayAverage returns the arithmetic average of normalized daily values.
func DisplayAverage[T interface{ Stamp | StampGroup }](stamps []T) float64 {
	if len(stamps) == 0 {
		return 0
	}

	var total float64
	for _, value := range stamps {
		switch stamp := any(value).(type) {
		case Stamp:
			total += stamp.AtRestTotalBytes
		case StampGroup:
			total += stamp.AtRestTotalBytes
		}
	}
	return total / float64(len(stamps))
}

func validSatelliteStamp(stamp Stamp) bool {
	return stamp.AtRestTotal > 0 && !stamp.IntervalEndTime.IsZero()
}

func calculatedStamp(satelliteID storj.NodeID, day time.Time, rate, hours float64) Stamp {
	return Stamp{
		SatelliteID:      satelliteID,
		AtRestTotalBytes: rate,
		IntervalInHours:  hours,
		IntervalStart:    day,
		Calculated:       true,
	}
}

func utcDay(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
