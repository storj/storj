// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reports_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/cmd/satellite/reports"
	"storj.io/storj/satellite/attribution"
)

func TestSumAttributionByUserAgent(t *testing.T) {
	log := zaptest.NewLogger(t)
	// test empty user agents
	attributions := []*attribution.BucketUsage{
		{
			UserAgent:    []byte{},
			ByteHours:    1,
			SegmentHours: 1,
			ObjectHours:  1,
			Hours:        1,
			EgressData:   1,
		},
		{
			ByteHours:    1,
			SegmentHours: 1,
			ObjectHours:  1,
			Hours:        1,
			EgressData:   1,
		},
	}
	totals := reports.SumAttributionByUserAgent(attributions, log)
	require.Equal(t, 0, len(totals))

	// test user agent with additional entries and uppercase letters is summed with
	// the first one
	attributions = []*attribution.BucketUsage{
		{
			UserAgent:    []byte("teststorj"),
			ByteHours:    1,
			SegmentHours: 1,
			ObjectHours:  1,
			Hours:        1,
			EgressData:   1,
		},
		{
			UserAgent:    []byte("TESTSTORJ/other"),
			ByteHours:    1,
			SegmentHours: 1,
			ObjectHours:  1,
			Hours:        1,
			EgressData:   1,
		},
	}
	totals = reports.SumAttributionByUserAgent(attributions, log)
	require.Equal(t, 1, len(totals))
	require.Equal(t, float64(2), totals["teststorj"].ByteHours)
	require.Equal(t, float64(2), totals["teststorj"].SegmentHours)
	require.Equal(t, float64(2), totals["teststorj"].ObjectHours)
	require.Equal(t, float64(2), totals["teststorj"].BucketHours)
	require.Equal(t, int64(2), totals["teststorj"].BytesEgress)

	// test two user agents are summed separately
	attributions = []*attribution.BucketUsage{
		{
			UserAgent:    []byte("teststorj1"),
			ByteHours:    1,
			SegmentHours: 1,
			ObjectHours:  1,
			Hours:        1,
			EgressData:   1,
		},
		{
			UserAgent:    []byte("teststorj1"),
			ByteHours:    1,
			SegmentHours: 1,
			ObjectHours:  1,
			Hours:        1,
			EgressData:   1,
		},
		{
			UserAgent:    []byte("teststorj2"),
			ByteHours:    1,
			SegmentHours: 1,
			ObjectHours:  1,
			Hours:        1,
			EgressData:   1,
		},
		{
			UserAgent:    []byte("teststorj2"),
			ByteHours:    1,
			SegmentHours: 1,
			ObjectHours:  1,
			Hours:        1,
			EgressData:   1,
		},
	}
	totals = reports.SumAttributionByUserAgent(attributions, log)
	require.Equal(t, 2, len(totals))
	require.Equal(t, float64(2), totals["teststorj1"].ByteHours)
	require.Equal(t, float64(2), totals["teststorj1"].SegmentHours)
	require.Equal(t, float64(2), totals["teststorj1"].ObjectHours)
	require.Equal(t, float64(2), totals["teststorj1"].BucketHours)
	require.Equal(t, int64(2), totals["teststorj1"].BytesEgress)
	require.Equal(t, float64(2), totals["teststorj2"].ByteHours)
	require.Equal(t, float64(2), totals["teststorj2"].SegmentHours)
	require.Equal(t, float64(2), totals["teststorj2"].ObjectHours)
	require.Equal(t, float64(2), totals["teststorj2"].BucketHours)
	require.Equal(t, int64(2), totals["teststorj2"].BytesEgress)
}
