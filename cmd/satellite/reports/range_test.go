// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reports_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/cmd/satellite/reports"
)

func TestParseRange(t *testing.T) {
	for _, tt := range []struct {
		name     string
		startIn  string
		endIn    string
		startOut time.Time
		endOut   time.Time
		err      string
	}{
		{
			name:     "range end is exclusive",
			startIn:  "2019-11-01",
			endIn:    "2019-12-01",
			startOut: time.Date(2019, 11, 01, 0, 0, 0, 0, time.UTC),
			endOut:   time.Date(2019, 12, 01, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "invalid start date",
			startIn: "BAD",
			endIn:   "2019-12-01",
			err:     "malformed start date (use YYYY-MM-DD)",
		},
		{
			name:    "invalid end date",
			startIn: "2019-11-01",
			endIn:   "BAD",
			err:     "malformed end date (use YYYY-MM-DD)",
		},
		{
			name:    "start date must come before end date",
			startIn: "2019-11-01",
			endIn:   "2019-11-01",
			err:     "invalid date range: start date must come before end date",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := reports.ParseRange(tt.startIn, tt.endIn)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.startOut, start)
			assert.Equal(t, tt.endOut, end)
		})
	}
}
