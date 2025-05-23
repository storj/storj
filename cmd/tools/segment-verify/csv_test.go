// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	segmentverify "storj.io/storj/cmd/tools/segment-verify"
	"storj.io/storj/satellite/metabase"
)

func TestCSVWriter(t *testing.T) {
	ctx := testcontext.New(t)

	var out strings.Builder
	w := segmentverify.NewCustomCSVWriter(&out)
	err := w.Write(ctx, []*segmentverify.Segment{
		{
			VerifySegment: metabase.VerifySegment{
				StreamID: uuid.UUID{1, 2, 3, 4, 5, 6},
				Position: metabase.SegmentPosition{Part: 10, Index: 56},
				Redundancy: storj.RedundancyScheme{
					RequiredShares: 6,
				},
			},
			Status: segmentverify.Status{Retry: 1, Found: 3, NotFound: 5},
		},
		{
			VerifySegment: metabase.VerifySegment{
				StreamID: uuid.UUID{10},
				Position: metabase.SegmentPosition{Part: 1, Index: 1},
				Redundancy: storj.RedundancyScheme{
					RequiredShares: 3,
				},
			},
			Status: segmentverify.Status{Retry: 1, Found: 3, NotFound: 0},
		},
	})
	require.NoError(t, err)

	now := time.Now()
	err = w.Write(ctx, []*segmentverify.Segment{
		{
			VerifySegment: metabase.VerifySegment{
				StreamID: uuid.UUID{11},
				Position: metabase.SegmentPosition{Part: 5, Index: 2},
				Redundancy: storj.RedundancyScheme{
					RequiredShares: 9,
				},
				CreatedAt: now,
			},
			Status: segmentverify.Status{Retry: 2, Found: 5, NotFound: 9},
		},
	})
	require.NoError(t, err)

	require.NoError(t, w.Close())

	require.Equal(t, ""+
		"stream id,position,created_at,required,found,not found,retry\n"+
		"01020304-0506-0000-0000-000000000000,42949673016,0001-01-01T00:00:00Z,6,3,5,1\n"+
		"0a000000-0000-0000-0000-000000000000,4294967297,0001-01-01T00:00:00Z,3,3,0,1\n"+
		"0b000000-0000-0000-0000-000000000000,21474836482,"+now.Format(time.RFC3339)+",9,5,9,2\n",
		out.String())
}
