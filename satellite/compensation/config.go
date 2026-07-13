// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"strconv"
	"strings"
)

// Config contains configuration for the calculations this package performs.
type Config struct {
	Rates            Rates
	WithheldPercents Percents `user:"true" help:"comma separated monthly withheld percentage rates" default:"75,75,75,50,50,50,25,25,25,0,0,0,0,0,0"`
	DisposePercent   int      `user:"true" help:"percent of held amount disposed to node after leaving withheld" default:"50"`
}

// Percents is used to hold a list of percentages, typically for the withheld schedule.
type Percents []int

// String formats the percentages.
func (percents Percents) String() string {
	s := make([]string, 0, len(percents))
	for _, percent := range percents {
		s = append(s, strconv.FormatInt(int64(percent), 10))
	}
	return strings.Join(s, ",")
}

// Set implements pflag.Value by parsing a comma separated list of percents.
func (percents *Percents) Set(value string) error {
	var entries []string
	if value != "" {
		entries = strings.Split(value, ",")
	}

	var toSet []int
	for _, entry := range entries {
		percent, err := strconv.ParseInt(entry, 10, 0)
		if err != nil {
			return Error.New("invalid percent %q: %w", entry, err)
		}
		toSet = append(toSet, int(percent))
	}

	*percents = toSet
	return nil
}

// Type returns the type of the pflag.Value.
func (percents Percents) Type() string {
	return "percents"
}
