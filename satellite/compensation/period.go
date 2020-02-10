// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"fmt"
	"time"
)

type Period struct {
	Year  int
	Month time.Month
}

func (p Period) String() string {
	return fmt.Sprintf("%04d-%02d", p.Year, p.Month)
}

func (p Period) StartDate() time.Time {
	return time.Date(p.Year, p.Month, 1, 0, 0, 0, 0, time.UTC)
}

func (p Period) EndDateExclusive() time.Time {
	return time.Date(p.Year, p.Month+1, 1, 0, 0, 0, 0, time.UTC)
}

func (p Period) Hours() int {
	return int(p.EndDateExclusive().Sub(p.StartDate()) / time.Hour)
}

func (p *Period) UnmarshalCSV(s string) error {
	v, err := PeriodFromString(s)
	if err != nil {
		return err
	}
	*p = v
	return nil
}

func (p Period) MarshalCSV() (string, error) {
	return p.String(), nil
}

func PeriodFromString(s string) (Period, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return Period{}, Error.Wrap(err)
	}
	return PeriodFromTime(t), nil
}

func PeriodFromTime(t time.Time) Period {
	year, month, _ := t.UTC().Date()
	return Period{
		Year:  year,
		Month: month,
	}
}
