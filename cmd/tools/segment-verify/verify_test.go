// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	segmentverify "storj.io/storj/cmd/tools/segment-verify"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metabase"
)

func TestVerifier(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		service := segmentverify.NewVerifier(
			planet.Log().Named("verifier"),
			satellite.Dialer,
			satellite.Orders.Service,
			segmentverify.VerifierConfig{
				PerPieceTimeout:    800 * time.Millisecond,
				OrderRetryThrottle: 50 * time.Millisecond,
			})

		// upload some data
		data := testrand.Bytes(8 * memory.KiB)
		for _, up := range planet.Uplinks {
			for i := 0; i < 10; i++ {
				err := up.Upload(ctx, satellite, "bucket1", strconv.Itoa(i), data)
				require.NoError(t, err)
			}
		}

		result, err := satellite.Metabase.DB.ListVerifySegments(ctx, metabase.ListVerifySegments{
			CursorStreamID: uuid.UUID{},
			CursorPosition: metabase.SegmentPosition{},
			Limit:          100000000,
		})
		require.NoError(t, err)

		validSegments := []*segmentverify.Segment{}
		for _, raw := range result.Segments {
			validSegments = append(validSegments, &segmentverify.Segment{
				VerifySegment: raw,
				Status:        segmentverify.Status{Retry: 1},
			})
		}

		// expect all segments are found on the node
		err = service.Verify(ctx, planet.StorageNodes[0].NodeURL(), validSegments)
		require.NoError(t, err)
		for _, seg := range validSegments {
			require.Equal(t, segmentverify.Status{Found: 1, NotFound: 0, Retry: 0}, seg.Status)
		}

		// segment not found
		missingSegment := &segmentverify.Segment{
			VerifySegment: metabase.VerifySegment{
				StreamID:    testrand.UUID(),
				Position:    metabase.SegmentPosition{},
				RootPieceID: testrand.PieceID(),
				AliasPieces: metabase.AliasPieces{{Number: 0, Alias: 1}},
			},
			Status: segmentverify.Status{Retry: 1},
		}

		err = service.Verify(ctx, planet.StorageNodes[0].NodeURL(), []*segmentverify.Segment{missingSegment})
		require.NoError(t, err)
		require.Equal(t, segmentverify.Status{Found: 0, NotFound: 1, Retry: 0}, missingSegment.Status)

		// TODO: test download timeout

		// node offline
		err = planet.StopNodeAndUpdate(ctx, planet.StorageNodes[0])
		require.NoError(t, err)
		err = service.Verify(ctx, planet.StorageNodes[0].NodeURL(), validSegments)
		require.Error(t, err)
		require.True(t, segmentverify.ErrNodeOffline.Has(err))
	})
}
