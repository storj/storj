// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze

import (
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// EmailIntervals is a list of durations representing
// how often freeze emails are sent for a freeze event.
type EmailIntervals []time.Duration

// Ensure that EmailIntervals implements pflag.Value.
var _ pflag.Value = (*EmailIntervals)(nil)

// String returns a comma-separated list of durations. e.g.: 24h,32m.
func (e *EmailIntervals) String() string {
	if e == nil {
		return ""
	}
	str := ""
	for i, d := range *e {
		if i > 0 {
			str += ","
		}
		str += d.String()
	}
	return str
}

// Set parses a comma-separated list of durations.
func (e *EmailIntervals) Set(s string) error {
	if s == "" {
		return nil
	}
	durationStrs := strings.Split(s, ",")
	ee := make(EmailIntervals, len(durationStrs))
	for i, durationStr := range durationStrs {
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			return err
		}
		ee[i] = d
	}
	*e = ee
	return nil
}

// Type returns the type of the pflag.Value.
func (e *EmailIntervals) Type() string {
	return "accountfreeze.EmailIntervals"
}
