// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze

import (
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// ConsoleConfig holds the console-level settings the freeze chores need.
type ConsoleConfig struct {
	ExternalAddress         string
	GeneralRequestURL       string
	FlagBots                bool
	TenantID                *string
	NewPricingEffectiveDate time.Time
	// LegacyPricingUserAgents are the user agents whose users keep legacy pricing and are
	// therefore exempt from the opt-in migration — the opt-out freeze chore skips them.
	LegacyPricingUserAgents []string
}

// Config contains configurable values for account freeze chore.
type Config struct {
	Enabled          bool          `help:"whether to run account freeze chores." default:"false"`
	Interval         time.Duration `help:"How often to run the account freeze chores." default:"24h"`
	PriceThreshold   int64         `help:"The failed invoice amount (in cents) beyond which an account will not be frozen" default:"100000"`
	ExcludeStorjscan bool          `help:"whether to exclude storjscan-paying users from automatic warn/freeze" default:"false"`

	OptOutFreezeBatchSize      int           `help:"How many users to fetch at a time to opt-out freeze." default:"100"`
	OptOutFreezeReminderBefore time.Duration `help:"how far before OptOutFreezeDate to send the pre-freeze reminder email; 0 disables the reminder" default:"0"`
	OptOutFreezeOptedOutOnly   bool          `help:"whether the opt-out freeze chore should only freeze users who explicitly opted out." default:"true"`

	UnattemptedInvoiceThreshold time.Duration `help:"how long an invoice can be unattempted before it triggers the Large-Invoice-Unpaid event" default:"24h"`

	EmailsEnabled                bool           `help:"whether to freeze event emails from this chore" default:"false"`
	BillingWarningEmailIntervals EmailIntervals `help:"how long to wait after a warning event to send reminder emails. E.g.: 1h,2h,3h will mean an email is sent 1h after the event, 2h after the event and 3h after the event" default:"240h,336h"`
	BillingFreezeEmailIntervals  EmailIntervals `help:"how long to wait after a freeze event to send reminder emails. E.g.: 1h,2h,3h will mean an email is sent 1h after the event, 2h after the event and 3h after the event" default:"720h,1200h,1416h"`

	InactivitySuspendEnabled        bool `help:"whether to enable inactivity-based account suspension." default:"false"`
	InactivityConsecutiveZeroCycles int  `help:"number of consecutive zero-usage billing cycles before issuing an inactivity warning." default:"3"`
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
