// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/accounting"
)

func TestNormalizeAcrossDayBoundaries(t *testing.T) {
	tests := []struct {
		name         string
		startTime    time.Time
		endTime      time.Time
		totalAmount  float64
		expectedDays int
		validate     func(t *testing.T, events []accounting.DailyUsageEvent)
	}{
		{
			name:         "single day",
			startTime:    time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			endTime:      time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC),
			totalAmount:  1000,
			expectedDays: 1,
			validate: func(t *testing.T, events []accounting.DailyUsageEvent) {
				assert.Equal(t, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), events[0].Date)
				assert.Equal(t, float64(1000), events[0].Amount)
			},
		},
		{
			name:         "end of day crossing",
			startTime:    time.Date(2025, 1, 15, 23, 0, 0, 0, time.UTC), // 11pm yesterday
			endTime:      time.Date(2025, 1, 16, 3, 0, 0, 0, time.UTC),  // 3am today
			totalAmount:  100,
			expectedDays: 2,
			validate: func(t *testing.T, events []accounting.DailyUsageEvent) {
				// Yesterday: 1 hour (23:00 to 00:00) out of 4 hours = 25% of 100 = 25
				assert.Equal(t, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), events[0].Date)
				assert.Equal(t, float64(25), events[0].Amount)

				// Today: 3 hours (00:00 to 03:00) out of 4 hours = 75% of 100 = 75
				assert.Equal(t, time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC), events[1].Date)
				assert.Equal(t, float64(75), events[1].Amount)
			},
		},
		{
			name:         "span three days",
			startTime:    time.Date(2025, 1, 15, 20, 0, 0, 0, time.UTC), // 8pm day 1
			endTime:      time.Date(2025, 1, 17, 4, 0, 0, 0, time.UTC),  // 4am day 3
			totalAmount:  3200,
			expectedDays: 3,
			validate: func(t *testing.T, events []accounting.DailyUsageEvent) {
				// Day 1: 4 hours (20:00 to 00:00) out of 32 hours = 12.5% of 3200 = 400
				assert.Equal(t, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), events[0].Date)
				assert.Equal(t, float64(400), events[0].Amount)

				// Day 2: 24 hours (full day) out of 32 hours = 75% of 3200 = 2400
				assert.Equal(t, time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC), events[1].Date)
				assert.Equal(t, float64(2400), events[1].Amount)

				// Day 3: 4 hours (00:00 to 04:00) out of 32 hours = 12.5% of 3200 = 400
				assert.Equal(t, time.Date(2025, 1, 17, 0, 0, 0, 0, time.UTC), events[2].Date)
				assert.Equal(t, float64(400), events[2].Amount)
			},
		},
		{
			name:         "exactly at midnight boundaries",
			startTime:    time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			endTime:      time.Date(2025, 1, 17, 0, 0, 0, 0, time.UTC),
			totalAmount:  4800,
			expectedDays: 2,
			validate: func(t *testing.T, events []accounting.DailyUsageEvent) {
				// Day 1: 24 hours = 50% of 4800 = 2400
				assert.Equal(t, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), events[0].Date)
				assert.Equal(t, float64(2400), events[0].Amount)

				// Day 2: 24 hours = 50% of 4800 = 2400
				assert.Equal(t, time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC), events[1].Date)
				assert.Equal(t, float64(2400), events[1].Amount)
			},
		},
		{
			name:         "invalid range",
			startTime:    time.Date(2025, 1, 16, 12, 0, 0, 0, time.UTC),
			endTime:      time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
			totalAmount:  1000,
			expectedDays: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := accounting.NormalizeAcrossDayBoundaries(tt.startTime, tt.endTime, tt.totalAmount)
			if tt.expectedDays == 0 {
				require.Error(t, err)
				require.Empty(t, events)
				return
			}
			require.NoError(t, err)
			assert.Len(t, events, tt.expectedDays)
			tt.validate(t, events)
		})
	}
}
