// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/shared/location"
)

func TestObserverForkProcess(t *testing.T) {

	nodes := func() (res []nodeselection.SelectedNode) {
		for i := 0; i < 10; i++ {
			res = append(res, nodeselection.SelectedNode{
				ID:          testidentity.MustPregeneratedIdentity(i, storj.LatestIDVersion()).ID,
				Online:      true,
				CountryCode: location.Germany,
				LastNet:     "127.0.0.0",
			})
		}
		return res
	}()

	mapNodes := func(nodes []nodeselection.SelectedNode, include func(node nodeselection.SelectedNode) bool) map[storj.NodeID]nodeselection.SelectedNode {
		res := map[storj.NodeID]nodeselection.SelectedNode{}
		for _, node := range nodes {
			if include(node) {
				res[node.ID] = node
			}
		}
		return res
	}

	ctx := testcontext.New(t)
	createDefaultObserver := func() *Observer {
		nodesCache := &ReliabilityCache{
			staleness: time.Hour,
		}
		o := &Observer{
			statsCollector: make(map[redundancyStyle]*observerRSStats),
			nodesCache:     nodesCache,
			placements:     nodeselection.TestPlacementDefinitions(),
			health:         NewProbabilityHealth(0.00005435, nodesCache),
		}

		o.nodesCache.state.Store(&reliabilityState{
			nodeByID: mapNodes(nodes, func(node nodeselection.SelectedNode) bool {
				return true
			}),
			created: time.Now(),
		})
		return o
	}

	createFork := func(o *Observer, q queue.RepairQueue) *observerFork {
		return &observerFork{
			log:              zaptest.NewLogger(t),
			getObserverStats: o.getObserverStats,
			rsStats:          make(map[redundancyStyle]*partialRSStats),
			doDeclumping:     o.doDeclumping,
			doPlacementCheck: o.doPlacementCheck,
			placements:       o.placements,
			getNodesEstimate: o.getNodesEstimate,
			nodesCache:       o.nodesCache,
			health:           o.health,
			repairQueue:      queue.NewInsertBuffer(q, 1000),
		}
	}

	createPieces := func(nodes []nodeselection.SelectedNode, selected ...int) metabase.Pieces {
		pieces := make(metabase.Pieces, len(selected))
		for ix, s := range selected {
			pieces[ix] = metabase.Piece{
				Number:      uint16(ix),
				StorageNode: nodes[s].ID,
			}
		}
		return pieces
	}

	t.Run("all healthy", func(t *testing.T) {
		o := createDefaultObserver()
		q := queue.MockRepairQueue{}
		fork := createFork(o, &q)
		err := fork.process(ctx, &rangedloop.Segment{
			Pieces: createPieces(nodes, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9),
			Redundancy: storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				ShareSize:      256,
				RepairShares:   4,
				RequiredShares: 6,
				OptimalShares:  8,
				TotalShares:    10,
			},
		})
		require.NoError(t, err)

		err = fork.repairQueue.Flush(ctx)
		require.NoError(t, err)

		require.Len(t, q.Segments, 0)

	})

	t.Run("declumping", func(t *testing.T) {
		o := createDefaultObserver()
		o.doDeclumping = true
		q := queue.MockRepairQueue{}
		fork := createFork(o, &q)
		err := fork.process(ctx, &rangedloop.Segment{
			Pieces: createPieces(nodes, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9),
			Redundancy: storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				ShareSize:      256,
				RepairShares:   4,
				RequiredShares: 6,
				OptimalShares:  8,
				TotalShares:    10,
			},
			RootPieceID: testrand.PieceID(),
		})
		require.NoError(t, err)

		err = fork.repairQueue.Flush(ctx)
		require.NoError(t, err)

		// as all test nodes are in the same subnet...
		require.Len(t, q.Segments, 1)
	})

	t.Run("declumping is ignored by annotation", func(t *testing.T) {
		o := createDefaultObserver()
		o.doDeclumping = true

		placements := nodeselection.ConfigurablePlacementRule{}
		require.NoError(t, placements.Set(fmt.Sprintf(`10:annotated(country("DE"),annotation("%s","%s"))`, nodeselection.AutoExcludeSubnet, nodeselection.AutoExcludeSubnetOFF)))
		parsed, err := placements.Parse(nil, nil)
		require.NoError(t, err)
		o.placements = parsed

		q := queue.MockRepairQueue{}
		fork := createFork(o, &q)
		err = fork.process(ctx, &rangedloop.Segment{
			Placement: 10,
			Pieces:    createPieces(nodes, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9),
			Redundancy: storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				ShareSize:      256,
				RepairShares:   4,
				RequiredShares: 6,
				OptimalShares:  8,
				TotalShares:    10,
			},
		})
		require.NoError(t, err)

		err = fork.repairQueue.Flush(ctx)
		require.NoError(t, err)

		require.Len(t, q.Segments, 0)
	})

}
