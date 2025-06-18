// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package internal

import (
	"encoding/json"
	"regexp"
	"strconv"
	"time"

	"github.com/zeebo/errs"
)

// ParseHumanDate parses command-line flags which accept relative and absolute datetimes.
// It can be passed to clingy.Transform to create a clingy.Option.
func ParseHumanDate(date string) (time.Time, error) {
	return ParseHumanDateInLocation(date, time.Now().Location(), RoundDown)
}

// ParseHumanDateNotBefore parses command-line flags which accept relative and absolute datetimes.
func ParseHumanDateNotBefore(date string) (time.Time, error) {
	return ParseHumanDateInLocation(date, time.Now().Location(), RoundDown)
}

// ParseHumanDateNotAfter parses relative/short date times. But it rounds up the period.
// For example ParseHumanDateNotAfter('2022-01-23') will return with '2022-01-23T23:59...' (end of day),
// and ParseHumanDateNotAfter('2022-01-23T15:04') will return with '2022-01-23T15:04:59...' (end of minute)...
func ParseHumanDateNotAfter(date string) (time.Time, error) {
	return ParseHumanDateInLocation(date, time.Now().Location(), RoundUp)
}

var durationWithDay = regexp.MustCompile(`(\+|-)(\d+)d`)

// RoundDirection represents the direction in which a value should be rounded.
type RoundDirection int

const (
	// RoundDown indicates that the value should be rounded down.
	RoundDown = RoundDirection(iota)

	// RoundUp indicates that the value should be rounded up.
	RoundUp
)

// ParseHumanDateInLocation parses relative and absolute datetimes in a given location.
// If an absolute datetime is given and ceil is true, then the returned time is rounded
// up to the end of the day, minute, or second, depending on the specificity of the input.
func ParseHumanDateInLocation(date string, loc *time.Location, roundDir RoundDirection) (time.Time, error) {
	var ceil bool
	switch roundDir {
	case RoundDown:
	case RoundUp:
		ceil = true
	default:
		return time.Time{}, errs.New("invalid round direction %d", roundDir)
	}

	switch {
	case date == "none":
		return time.Time{}, nil
	case date == "":
		return time.Time{}, nil
	case date == "now":
		return time.Now(), nil
	case date[0] == '+' || date[0] == '-':
		dayDuration := durationWithDay.FindStringSubmatch(date)
		if len(dayDuration) > 0 {
			days, _ := strconv.Atoi(dayDuration[2])
			if dayDuration[1] == "-" {
				days *= -1
			}
			return time.Now().Add(time.Hour * time.Duration(days*24)), nil
		}

		d, err := time.ParseDuration(date)
		return time.Now().Add(d), errs.Wrap(err)
	default:
		t, err := time.ParseInLocation(time.RFC3339, date, time.Now().Location())
		if err == nil {
			return t, nil
		}

		// shorter version of RFC3339
		t, err = time.ParseInLocation("2006-01-02T15:04:05", date, loc)
		if err == nil {
			if ceil {
				t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second()+1, -1, loc)
			}
			return t, nil
		}

		t, err = time.ParseInLocation("2006-01-02T15:04", date, loc)
		if err == nil {
			if ceil {
				t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()+1, 0, -1, loc)
			}
			return t, nil
		}

		t, err = time.ParseInLocation("2006-01-02", date, loc)
		if err == nil {
			if ceil {
				t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, -1, loc)
			}
			return t, nil
		}

		d, err := time.ParseDuration(date)
		if err == nil {
			return time.Now().Add(d), nil
		}
		return time.Time{}, err
	}
}

// ParseJSON parses command-line flags which accept JSON string.
// It can be passed to clingy.Transform to create a clingy.Option.
func ParseJSON(jsonString string) (map[string]string, error) {
	if len(jsonString) > 0 {
		var jsonValue map[string]string
		err := json.Unmarshal([]byte(jsonString), &jsonValue)
		if err != nil {
			return nil, err
		}
		return jsonValue, nil
	}
	return nil, nil
}
