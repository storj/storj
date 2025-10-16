// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
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
	const (
		nodeCount   = 10
		uplinkCount = 10
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: nodeCount, UplinkCount: uplinkCount,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(nodeCount, nodeCount, nodeCount, nodeCount),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		config := segmentverify.VerifierConfig{
			PerPieceTimeout:    10 * time.Second,
			OrderRetryThrottle: 500 * time.Millisecond,
			RequestThrottle:    500 * time.Millisecond,
		}

		// create new observed logger
		observedZapCore, observedLogs := observer.New(zap.DebugLevel)
		observedLogger := zap.New(observedZapCore).Named("verifier")

		verifier := segmentverify.NewVerifier(
			observedLogger,
			satellite.Dialer,
			satellite.Orders.Service,
			config)

		// upload some data
		data := testrand.Bytes(8 * memory.KiB)
		for u, up := range planet.Uplinks {
			for i := 0; i < nodeCount; i++ {
				err := up.Upload(ctx, satellite, "bucket1", fmt.Sprintf("uplink%d/i%d", u, i), data)
				require.NoError(t, err)
			}
		}

		result, err := satellite.Metabase.DB.ListVerifySegments(ctx, metabase.ListVerifySegments{
			CursorStreamID: uuid.UUID{},
			CursorPosition: metabase.SegmentPosition{},
			Limit:          10000,
		})
		require.NoError(t, err)
		require.Len(t, result.Segments, uplinkCount*nodeCount)

		validSegments := make([]*segmentverify.Segment, len(result.Segments))
		for i, raw := range result.Segments {
			validSegments[i] = &segmentverify.Segment{VerifySegment: raw}
		}

		resetStatuses := func() {
			for _, seg := range validSegments {
				seg.Status = segmentverify.Status{Retry: nodeCount}
			}
		}
		resetStatuses()

		aliasMap, err := satellite.Metabase.DB.LatestNodesAliasMap(ctx)
		require.NoError(t, err)

		t.Run("verify all", func(t *testing.T) {
			var g errgroup.Group
			for _, node := range planet.StorageNodes {
				node := node
				alias, ok := aliasMap.Alias(node.ID())
				require.True(t, ok)
				g.Go(func() error {
					_, err := verifier.Verify(ctx, alias, node.NodeURL(), validSegments, true)
					return err
				})
			}
			require.NoError(t, g.Wait())
			require.NotZero(t, len(observedLogs.All()))

			for segNum, seg := range validSegments {
				require.Equal(t, segmentverify.Status{Found: nodeCount, NotFound: 0, Retry: 0}, seg.Status, segNum)
			}
		})

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
		t.Run("segment not found using download method", func(t *testing.T) {
			// for older node version
			count, err = verifier.Verify(ctx, alias0, planet.StorageNodes[0].NodeURL(),
				[]*segmentverify.Segment{validSegment0, missingSegment, validSegment1}, true)
			require.NoError(t, err)
			require.Equal(t, 3, count)
			require.Equal(t, segmentverify.Status{Found: 1}, validSegment0.Status)
			require.Equal(t, segmentverify.Status{NotFound: 1}, missingSegment.Status)
			require.Equal(t, segmentverify.Status{Found: 1}, validSegment1.Status)
		})

		resetStatuses()

		t.Run("test throttling", func(t *testing.T) {
			// Test throttling
			verifyStart := time.Now()
			const throttleN = 5
			count, err = verifier.Verify(ctx, alias0, planet.StorageNodes[0].NodeURL(), validSegments[:throttleN], false)
			require.NoError(t, err)
			verifyDuration := time.Since(verifyStart)
			require.Equal(t, throttleN, count)
			require.Greater(t, verifyDuration, config.RequestThrottle*(throttleN-1))
		})

		resetStatuses()

		// TODO: test download timeout
		t.Run("Node offline", func(t *testing.T) {
			err = planet.StopNodeAndUpdate(ctx, planet.StorageNodes[0])
			require.NoError(t, err)

			// for older node version
			count, err = verifier.Verify(ctx, alias0, planet.StorageNodes[0].NodeURL(), validSegments, true)
			require.Error(t, err)
			require.Equal(t, 0, count)
			require.True(t, segmentverify.ErrNodeOffline.Has(err))

			// for node version with Exists endpoint
			count, err = verifier.Verify(ctx, alias0, planet.StorageNodes[0].NodeURL(), validSegments, true)
			require.Error(t, err)
			require.Equal(t, 0, count)
			require.True(t, segmentverify.ErrNodeOffline.Has(err))
		})
	})
}
