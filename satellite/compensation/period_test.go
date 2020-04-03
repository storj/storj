// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPeriod(t *testing.T) {
	for _, tt := range []struct {
		year      int
		month     time.Month
		startDate string
		endDate   string
		days      int
	}{
		{year: 2019, month: 1, startDate: "2019-01-01", endDate: "2019-02-01", days: 31},
		{year: 2019, month: 2, startDate: "2019-02-01", endDate: "2019-03-01", days: 28},
		{year: 2019, month: 3, startDate: "2019-03-01", endDate: "2019-04-01", days: 31},
		{year: 2019, month: 4, startDate: "2019-04-01", endDate: "2019-05-01", days: 30},
		{year: 2019, month: 5, startDate: "2019-05-01", endDate: "2019-06-01", days: 31},
		{year: 2019, month: 6, startDate: "2019-06-01", endDate: "2019-07-01", days: 30},
		{year: 2019, month: 7, startDate: "2019-07-01", endDate: "2019-08-01", days: 31},
		{year: 2019, month: 8, startDate: "2019-08-01", endDate: "2019-09-01", days: 31},
		{year: 2019, month: 9, startDate: "2019-09-01", endDate: "2019-10-01", days: 30},
		{year: 2019, month: 10, startDate: "2019-10-01", endDate: "2019-11-01", days: 31},
		{year: 2019, month: 11, startDate: "2019-11-01", endDate: "2019-12-01", days: 30},
		{year: 2019, month: 12, startDate: "2019-12-01", endDate: "2020-01-01", days: 31},
		// leap year/month
		{year: 2020, month: 2, startDate: "2020-02-01", endDate: "2020-03-01", days: 29},
	} {
		t.Logf("year:%d month:%d startDate:%s endDate:%s days:%d", tt.year, tt.month, tt.startDate, tt.endDate, tt.days)

		period := Period{Year: tt.year, Month: tt.month}
		assert.Equal(t, tt.startDate, period.StartDate().Format("2006-01-02"))
		assert.Equal(t, tt.endDate, period.EndDateExclusive().Format("2006-01-02"))
	}
}
