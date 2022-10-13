// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase/segmentloop"
)

// TestAuditCollector does the following:
// - start testplanet with 5 nodes and a reservoir size of 3
// - upload 5 files
// - iterate over all the segments in satellite.Metainfo and store them in allPieces map
// - create a audit observer and call metaloop.Join(auditObs)
//
// Then for every node in testplanet:
//   - expect that there is a reservoir for that node on the audit observer
//   - that the reservoir size is <= 2 (the maxReservoirSize)
//   - that every item in the reservoir is unique
func TestAuditCollector(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(3, 4, 5, 5),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		satellite.Audit.Worker.Loop.Pause()

		ul := planet.Uplinks[0]

		// upload 5 remote files with 1 segment
		for i := 0; i < 5; i++ {
			testData := testrand.Bytes(8 * memory.KiB)
			path := "/some/remote/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "testbucket", path, testData)
			require.NoError(t, err)
		}

		r := rand.New(rand.NewSource(time.Now().Unix()))
		observer := audit.NewCollector(4, r)
		err := satellite.Metabase.SegmentLoop.Join(ctx, observer)
		require.NoError(t, err)

		for _, node := range planet.StorageNodes {
			// expect a reservoir for every node
			require.NotNil(t, observer.Reservoirs[node.ID()])
			require.True(t, len(observer.Reservoirs[node.ID()].Segments) > 1)

			// Require that len segments are <= 3 even though the Collector was instantiated with 4
			// because the maxReservoirSize is currently 3.
			require.True(t, len(observer.Reservoirs[node.ID()].Segments) <= 3)

			repeats := make(map[audit.Segment]bool)
			for _, loopSegment := range observer.Reservoirs[node.ID()].Segments {
				segment := audit.NewSegment(loopSegment)
				assert.False(t, repeats[segment], "expected every item in reservoir to be unique")
				repeats[segment] = true
			}
		}
	})
}

func BenchmarkRemoteSegment(b *testing.B) {
	testplanet.Bench(b, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(b *testing.B, ctx *testcontext.Context, planet *testplanet.Planet) {

		for i := 0; i < 10; i++ {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object"+strconv.Itoa(i), testrand.Bytes(10*memory.KiB))
			require.NoError(b, err)
		}

		observer := audit.NewCollector(3, rand.New(rand.NewSource(time.Now().Unix())))

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(b, err)

		loopSegments := []*segmentloop.Segment{}

		for _, segment := range segments {
			loopSegments = append(loopSegments, &segmentloop.Segment{
				StreamID:   segment.StreamID,
				Position:   segment.Position,
				CreatedAt:  segment.CreatedAt,
				ExpiresAt:  segment.ExpiresAt,
				Redundancy: segment.Redundancy,
				Pieces:     segment.Pieces,
			})
		}

		b.Run("multiple segments", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, loopSegment := range loopSegments {
					err := observer.RemoteSegment(ctx, loopSegment)
					if err != nil {
						b.FailNow()
					}
				}
			}
		})
	})

}
