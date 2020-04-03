// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"fmt"
	"time"
)

// Period represents a monthly payment period.
type Period struct {
	Year  int
	Month time.Month
}

// String outputs the YYYY-MM form of the payment period.
func (p Period) String() string {
	return fmt.Sprintf("%04d-%02d", p.Year, p.Month)
}

// StartDate returns a time.Time that is less than or equal to any time in the period.
func (p Period) StartDate() time.Time {
	return time.Date(p.Year, p.Month, 1, 0, 0, 0, 0, time.UTC)
}

// EndDateExclusive returns a time.Time that is greater than any time in the period.
func (p Period) EndDateExclusive() time.Time {
	return time.Date(p.Year, p.Month+1, 1, 0, 0, 0, 0, time.UTC)
}

// UnmarshalCSV reads the Period in CSV form.
func (p *Period) UnmarshalCSV(s string) error {
	v, err := PeriodFromString(s)
	if err != nil {
		return err
	}
	*p = v
	return nil
}

// MarshalCSV returns the CSV form of the Period.
func (p Period) MarshalCSV() (string, error) {
	return p.String(), nil
}

// PeriodFromString parses the YYYY-MM string into a Period.
func PeriodFromString(s string) (Period, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return Period{}, Error.Wrap(err)
	}
	return PeriodFromTime(t), nil
}

// PeriodFromTime takes a time.Time and returns a Period that contains it.
func PeriodFromTime(t time.Time) Period {
	year, month, _ := t.UTC().Date()
	return Period{
		Year:  year,
		Month: month,
	}
}
