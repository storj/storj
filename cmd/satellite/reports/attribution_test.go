// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reports_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/uuid"
	"storj.io/storj/cmd/satellite/reports"
	"storj.io/storj/satellite/attribution"
)

func TestProcessAttributions(t *testing.T) {
	log := zaptest.NewLogger(t)

	requireSum := func(total reports.Total, n int) {
		require.Equal(t, float64(n), total.ByteHours)
		require.Equal(t, float64(n), total.SegmentHours)
		require.Equal(t, float64(n), total.ObjectHours)
		require.Equal(t, float64(n), total.BucketHours)
		require.Equal(t, int64(n), total.BytesEgress)
	}

	newUsage := func(userAgent string, projectID uuid.UUID, bucketName string) *attribution.BucketUsage {
		return &attribution.BucketUsage{
			UserAgent:    []byte(userAgent),
			ProjectID:    projectID.Bytes(),
			BucketName:   []byte(bucketName),
			ByteHours:    1,
			SegmentHours: 1,
			ObjectHours:  1,
			Hours:        1,
			EgressData:   1,
		}
	}

	id, err := uuid.New()
	require.NoError(t, err)

	// test empty user agents
	attributions := []*attribution.BucketUsage{
		newUsage("", id, ""),
		{
			ByteHours:    1,
			SegmentHours: 1,
			ObjectHours:  1,
			Hours:        1,
			EgressData:   1,
		},
	}
	totals := reports.ProcessAttributions(attributions, nil, log)
	require.Equal(t, 0, len(totals))

	// test user agent with additional entries and uppercase letters is summed with
	// the first one
	attributions = []*attribution.BucketUsage{
		newUsage("teststorj", id, ""),
		newUsage("TESTSTORJ/other", id, ""),
	}
	totals = reports.ProcessAttributions(attributions, nil, log)
	require.Equal(t, 1, len(totals))
	requireSum(totals[reports.AttributionTotalsIndex{"teststorj", id.String(), ""}], 2)

	// test two user agents are summed separately
	attributions = []*attribution.BucketUsage{
		newUsage("teststorj1", id, ""),
		newUsage("teststorj1", id, ""),
		newUsage("teststorj2", id, ""),
		newUsage("teststorj2", id, ""),
	}
	totals = reports.ProcessAttributions(attributions, nil, log)
	require.Equal(t, 2, len(totals))
	requireSum(totals[reports.AttributionTotalsIndex{"teststorj1", id.String(), ""}], 2)
	requireSum(totals[reports.AttributionTotalsIndex{"teststorj2", id.String(), ""}], 2)

	// Test that different project IDs are summed separately
	id2, err := uuid.New()
	require.NoError(t, err)
	attributions = []*attribution.BucketUsage{
		newUsage("teststorj1", id, ""),
		newUsage("teststorj1", id, ""),
		newUsage("teststorj1", id2, ""),
	}
	totals = reports.ProcessAttributions(attributions, nil, log)
	require.Equal(t, 2, len(totals))
	requireSum(totals[reports.AttributionTotalsIndex{"teststorj1", id.String(), ""}], 2)
	requireSum(totals[reports.AttributionTotalsIndex{"teststorj1", id2.String(), ""}], 1)

	// Test that different bucket names are summed separately
	attributions = []*attribution.BucketUsage{
		newUsage("teststorj1", id, "1"),
		newUsage("teststorj1", id, "1"),
		newUsage("teststorj1", id, "2"),
	}
	totals = reports.ProcessAttributions(attributions, nil, log)
	require.Equal(t, 2, len(totals))
	requireSum(totals[reports.AttributionTotalsIndex{"teststorj1", id.String(), "1"}], 2)
	requireSum(totals[reports.AttributionTotalsIndex{"teststorj1", id.String(), "2"}], 1)

	// Test that unspecified user agents are filtered out
	attributions = []*attribution.BucketUsage{
		newUsage("teststorj1", id, ""),
		newUsage("teststorj2", id, ""),
		newUsage("teststorj3", id, ""),
	}
	totals = reports.ProcessAttributions(attributions, []string{"teststorj1", "teststorj3"}, log)
	require.Equal(t, 2, len(totals))
	require.Contains(t, totals, reports.AttributionTotalsIndex{"teststorj1", id.String(), ""})
	require.Contains(t, totals, reports.AttributionTotalsIndex{"teststorj3", id.String(), ""})
}
