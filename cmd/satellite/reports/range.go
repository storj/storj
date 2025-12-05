// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reports

import (
	"time"

	"github.com/zeebo/errs"
)

// ParseRange parses report date range arguments. If the dates are malformed or
// if the start date does not come before the end date, an error is returned.
// The end date is exclusive.
func ParseRange(startArg, endArg string) (time.Time, time.Time, error) {
	layout := "2006-01-02"
	start, err := time.Parse(layout, startArg)
	if err != nil {
		return time.Time{}, time.Time{}, errs.New("malformed start date (use YYYY-MM-DD)")
	}
	end, err := time.Parse(layout, endArg)
	if err != nil {
		return time.Time{}, time.Time{}, errs.New("malformed end date (use YYYY-MM-DD)")
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, errs.New("invalid date range: start date must come before end date")
	}

	return start, end, nil
}
