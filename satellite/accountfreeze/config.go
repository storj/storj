// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze

import (
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// Config contains configurable values for account freeze chore.
type Config struct {
	Enabled          bool          `help:"whether to run this chore." default:"false"`
	Interval         time.Duration `help:"How often to run this chore, which is how often unpaid invoices are checked." default:"24h"`
	PriceThreshold   int64         `help:"The failed invoice amount (in cents) beyond which an account will not be frozen" default:"100000"`
	ExcludeStorjscan bool          `help:"whether to exclude storjscan-paying users from automatic warn/freeze" default:"false"`

	EmailsEnabled                bool           `help:"whether to freeze event emails from this chore" default:"false"`
	BillingWarningEmailIntervals EmailIntervals `help:"how long to wait after a warning event to send reminder emails. E.g.: 1h,2h,3h will mean an email is sent 1h after the event, 2h after the event and 3h after the event" default:"240h,336h"`
	BillingFreezeEmailIntervals  EmailIntervals `help:"how long to wait after a freeze event to send reminder emails. E.g.: 1h,2h,3h will mean an email is sent 1h after the event, 2h after the event and 3h after the event" default:"720h,1200h,1416h"`
}

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
