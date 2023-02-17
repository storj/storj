// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"flag"
	"regexp"
	"strconv"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
)

type stdlibFlags struct {
	fs *flag.FlagSet
}

func newStdlibFlags(fs *flag.FlagSet) *stdlibFlags {
	return &stdlibFlags{
		fs: fs,
	}
}

func (s *stdlibFlags) Setup(f clingy.Flags) {
	// we use the Transform function to store the value as a side
	// effect so that we can return an error if one occurs through
	// the expected clingy pipeline.
	s.fs.VisitAll(func(fl *flag.Flag) {
		name, _ := flag.UnquoteUsage(fl)
		f.Flag(fl.Name, fl.Usage, fl.DefValue,
			clingy.Advanced,
			clingy.Type(name),
			clingy.Transform(func(val string) (string, error) {
				return "", fl.Value.Set(val)
			}),
		)
	})
}

// parseHumanDate parses command-line flags which accept relative and absolute datetimes.
// It can be passed to clingy.Transform to create a clingy.Option.
func parseHumanDate(date string) (time.Time, error) {
	return parseHumanDateInLocation(date, time.Now().Location(), false)
}

// parseHumanDateNotBefore parses command-line flags which accept relative and absolute datetimes.
func parseHumanDateNotBefore(date string) (time.Time, error) {
	return parseHumanDateInLocation(date, time.Now().Location(), false)
}

// parseHumanDateNotAfter parses relative/short date times. But it rounds up the period.
// For example parseHumanDateNotAfter('2022-01-23') will return with '2022-01-23T23:59...' (end of day),
// and parseHumanDateNotAfter('2022-01-23T15:04') will return with '2022-01-23T15:04:59...' (end of minute)...
func parseHumanDateNotAfter(date string) (time.Time, error) {
	return parseHumanDateInLocation(date, time.Now().Location(), true)
}

var durationWithDay = regexp.MustCompile(`(\+|-)(\d+)d`)

func parseHumanDateInLocation(date string, loc *time.Location, ceil bool) (time.Time, error) {
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

// parseJSON parses command-line flags which accept JSON string.
// It can be passed to clingy.Transform to create a clingy.Option.
func parseJSON(jsonString string) (map[string]string, error) {
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
