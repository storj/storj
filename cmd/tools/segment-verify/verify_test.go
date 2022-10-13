// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

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
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(4, 4, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		snoCount := int32(len(planet.StorageNodes))

		config := segmentverify.VerifierConfig{
			PerPieceTimeout:    time.Second,
			OrderRetryThrottle: 500 * time.Millisecond,
			RequestThrottle:    500 * time.Millisecond,
		}
		service := segmentverify.NewVerifier(
			planet.Log().Named("verifier"),
			satellite.Dialer,
			satellite.Orders.Service,
			config)

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
			Limit:          10000,
		})
		require.NoError(t, err)

		validSegments := []*segmentverify.Segment{}
		for _, raw := range result.Segments {
			validSegments = append(validSegments, &segmentverify.Segment{
				VerifySegment: raw,
				Status:        segmentverify.Status{Retry: snoCount},
			})
		}

		aliasMap, err := satellite.Metabase.DB.LatestNodesAliasMap(ctx)
		require.NoError(t, err)

		var g errgroup.Group
		for _, node := range planet.StorageNodes {
			node := node
			alias, ok := aliasMap.Alias(node.ID())
			require.True(t, ok)
			g.Go(func() error {
				_, err := service.Verify(ctx, alias, node.NodeURL(), validSegments, true)
				return err
			})
		}
		require.NoError(t, g.Wait())
		for _, seg := range validSegments {
			require.Equal(t, segmentverify.Status{Found: snoCount, NotFound: 0, Retry: 0}, seg.Status)
		}

		// segment not found
		alias0, ok := aliasMap.Alias(planet.StorageNodes[0].ID())
		require.True(t, ok)

		validSegment0 := &segmentverify.Segment{
			VerifySegment: result.Segments[0],
			Status:        segmentverify.Status{Retry: 1},
		}
		missingSegment := &segmentverify.Segment{
			VerifySegment: metabase.VerifySegment{
				StreamID:    testrand.UUID(),
				Position:    metabase.SegmentPosition{},
				RootPieceID: testrand.PieceID(),
				Redundancy:  result.Segments[0].Redundancy,
				AliasPieces: metabase.AliasPieces{{Number: 0, Alias: alias0}},
			},
			Status: segmentverify.Status{Retry: 1},
		}
		validSegment1 := &segmentverify.Segment{
			VerifySegment: result.Segments[1],
			Status:        segmentverify.Status{Retry: 1},
		}

		var count int
		count, err = service.Verify(ctx, alias0, planet.StorageNodes[0].NodeURL(),
			[]*segmentverify.Segment{validSegment0, missingSegment, validSegment1}, true)
		require.NoError(t, err)
		require.Equal(t, 3, count)
		require.Equal(t, segmentverify.Status{Found: 1}, validSegment0.Status)
		require.Equal(t, segmentverify.Status{NotFound: 1}, missingSegment.Status)
		require.Equal(t, segmentverify.Status{Found: 1}, validSegment1.Status)

		// Test throttling
		verifyStart := time.Now()
		const throttleN = 5
		count, err = service.Verify(ctx, alias0, planet.StorageNodes[0].NodeURL(), validSegments[:throttleN], false)
		require.NoError(t, err)
		verifyDuration := time.Since(verifyStart)
		require.Equal(t, throttleN, count)
		require.Greater(t, verifyDuration, config.RequestThrottle*(throttleN-1))

		// TODO: test download timeout

		// node offline
		err = planet.StopNodeAndUpdate(ctx, planet.StorageNodes[0])
		require.NoError(t, err)
		count, err = service.Verify(ctx, alias0, planet.StorageNodes[0].NodeURL(), validSegments, true)
		require.Error(t, err)
		require.Equal(t, 0, count)
		require.True(t, segmentverify.ErrNodeOffline.Has(err))
	})
}
