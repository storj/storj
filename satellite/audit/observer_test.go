// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/storagenode"
)

// TestAuditCollector does the following:
// - start testplanet with 6 nodes and a reservoir size of 3
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
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(3, 4, 6, 6),
			StorageNode: func(index int, config *storagenode.Config) {
				if index == 5 {
					config.Contact.SelfSignedTags = []string{"ignore=true"}
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()

		ul := planet.Uplinks[0]

		for i := range 5 {
			testData := testrand.Bytes(8 * memory.KiB)
			path := "/some/remote/path/" + strconv.Itoa(i)
			require.NoError(t, ul.Upload(ctx, satellite, "testbucket", path, testData))
		}

		filter, err := nodeselection.FilterFromString(fmt.Sprintf(`exclude(tag("%s","ignore","true"))`, planet.StorageNodes[5].ID()), nil)
		require.NoError(t, err)

		include := audit.NewFilteredNodes(filter, satellite.DB.OverlayCache(), satellite.Metabase.DB)

		observer := audit.NewObserver(zaptest.NewLogger(t), include, satellite.Audit.VerifyQueue, satellite.Config.Audit)

		ranges := rangedloop.NewMetabaseRangeSplitter(zaptest.NewLogger(t), satellite.Metabase.DB, rangedloop.Config{
			BatchSize:               100,
			TestingSpannerQueryType: "read",
		})
		loop := rangedloop.NewService(zaptest.NewLogger(t), satellite.Config.RangedLoop, ranges, []rangedloop.Observer{observer})
		_, err = loop.RunOnce(ctx)
		require.NoError(t, err)

		aliases, err := planet.Satellites[0].Metabase.DB.LatestNodesAliasMap(ctx)
		require.NoError(t, err)

		// No reservoir for the last nodes, excluded by the filter
		lastID := planet.StorageNodes[5].ID()
		lastAlias, ok := aliases.Alias(lastID)
		require.True(t, ok)
		require.Nil(t, observer.Reservoirs[lastAlias])

		for _, node := range planet.StorageNodes[:5] {
			nodeID, ok := aliases.Alias(node.ID())
			require.True(t, ok)

			// expect a reservoir for every node
			require.NotNil(t, observer.Reservoirs[nodeID])
			require.True(t, len(observer.Reservoirs[nodeID].Segments()) > 1)

			// Require that len segments are <= 3 even though the Collector was instantiated with 4
			// because the maxReservoirSize is currently 3.
			require.True(t, len(observer.Reservoirs[nodeID].Segments()) <= 3)

			repeats := make(map[audit.Segment]bool)
			for _, segment := range observer.Reservoirs[nodeID].Segments() {
				assert.False(t, repeats[segment], "expected every item in reservoir to be unique")
				repeats[segment] = true
			}
		}
	})
}

func BenchmarkRemoteSegment(b *testing.B) {
	ctx := testcontext.New(b)

	observer := audit.NewObserver(zap.NewNop(), nil, nil, audit.Config{
		Slots: 3,
	})

	loopSegments := make([]rangedloop.Segment, 10000)
	for i := range loopSegments {
		loopSegments[i] = rangedloop.Segment{
			StreamID: testrand.UUID(),
			Redundancy: storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				RequiredShares: 1,
				TotalShares:    1,
			},
			RootPieceID: testrand.PieceID(),
		}

		for j := range 10 {
			loopSegments[i].AliasPieces = append(loopSegments[i].AliasPieces, metabase.AliasPiece{
				Number: uint16(j),
				Alias:  metabase.NodeAlias(i + j),
			})
		}
	}

	fork, err := observer.Fork(ctx)
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchSegments := loopSegments
		for len(benchSegments) > 0 {
			batch := benchSegments[:1000]
			err := fork.Process(ctx, batch)
			require.NoError(b, err)
			benchSegments = benchSegments[1000:]
		}
	}
}
