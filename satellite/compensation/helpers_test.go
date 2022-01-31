// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/satellite/compensation"
)

func TestNodeWithheldPercent(t *testing.T) {
	date := func(year int, month time.Month, day int) time.Time {
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	}

	rates := []int{75, 75, 75, 50, 50, 50, 25, 25, 25, 0, 0, 0, 0, 0, 0}

	startDate := date(2019, 1, 1)

	for _, tt := range []struct {
		rate          int
		inWithholding bool
		date          time.Time
	}{
		{rate: 75, inWithholding: true, date: startDate},
		{rate: 75, inWithholding: true, date: date(2019, 2, 1)},
		{rate: 75, inWithholding: true, date: date(2019, 3, 1)},
		{rate: 75, inWithholding: true, date: date(2019, 3, 31)},
		{rate: 50, inWithholding: true, date: date(2019, 4, 1)},
		{rate: 50, inWithholding: true, date: date(2019, 5, 1)},
		{rate: 50, inWithholding: true, date: date(2019, 6, 1)},
		{rate: 50, inWithholding: true, date: date(2019, 6, 30)},
		{rate: 25, inWithholding: true, date: date(2019, 7, 1)},
		{rate: 25, inWithholding: true, date: date(2019, 8, 1)},
		{rate: 25, inWithholding: true, date: date(2019, 9, 1)},
		{rate: 25, inWithholding: true, date: date(2019, 9, 30)},
		{rate: 00, inWithholding: true, date: date(2019, 10, 1)},
		{rate: 00, inWithholding: true, date: date(2019, 11, 1)},
		{rate: 00, inWithholding: true, date: date(2019, 12, 1)},
		{rate: 00, inWithholding: true, date: date(2020, 1, 1)},
		{rate: 00, inWithholding: true, date: date(2020, 2, 1)},
		{rate: 00, inWithholding: true, date: date(2020, 3, 1)},
		{rate: 00, inWithholding: true, date: date(2020, 3, 31)},
		{rate: 00, inWithholding: false, date: date(2020, 4, 1)},
	} {
		t.Logf("rate=%d inWithholding=%t date=%s", tt.rate, tt.inWithholding, tt.date.Format("2006-01"))
		rate, inWithholding := compensation.NodeWithheldPercent(rates, startDate, tt.date)
		assert.Equal(t, tt.rate, rate)
		assert.Equal(t, tt.inWithholding, inWithholding)
	}
}

func TestPercentOf(t *testing.T) {
	percentOf := func(v, p int64) int64 {
		return compensation.PercentOf(decimal.NewFromInt(v), decimal.NewFromInt(p)).IntPart()
	}
	assert.Equal(t, int64(40), percentOf(200, 20))
	assert.Equal(t, int64(0), percentOf(200, 0))
	assert.Equal(t, int64(600), percentOf(200, 300))
}
