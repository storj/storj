// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stretchr/testify/assert"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/shared/location"
	"storj.io/storj/shared/mud"
	"storj.io/storj/shared/mud/mudtest"
)

func TestSuccessTrackerMonitor_Stats(t *testing.T) {
	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	node3 := testrand.NodeID()
	node4 := testrand.NodeID()

	mudtest.Run[*metainfo.SuccessTrackerMonitor](t,
		mudtest.WithTestLogger(t, func(ball *mud.Ball) {
			mud.Provide[overlay.DB](ball, func() overlay.DB {
				return &mockOverlayDB{
					nodes: []nodeselection.SelectedNode{
						{ID: node1, Online: true, CountryCode: location.UnitedStates},
						{ID: node2, Online: false, CountryCode: location.UnitedStates},
						{ID: node3, Online: true, CountryCode: location.Germany},
					},
				}
			})
			mud.Provide[metainfo.Config](ball, func() metainfo.Config {
				return metainfo.Config{
					SuccessTrackerMonitorEnabled: true,
					SuccessTrackerMonitorFilter:  `country("US")`,
				}
			})
			mud.Provide[*metainfo.SuccessTrackerMonitor](ball, metainfo.NewSuccessTrackerMonitor)
		}),
		func(ctx context.Context, t *testing.T, monitor *metainfo.SuccessTrackerMonitor) {
			go func() {
				_ = monitor.Run(t.Context())
			}()
			tracker := newMockSuccessTracker()
			tracker.scores[node1] = 0.95
			tracker.scores[node2] = 0.85
			tracker.scores[node3] = 0.75 // German node
			tracker.scores[node4] = 0.75 // node with succes score, but not in the cache. Might be new joiners.

			key := monkit.NewSeriesKey("test_tracker")
			monitor.RegisterTracker(key, tracker)

			collected := make(map[string]float64)
			monitor.Stats(func(key monkit.SeriesKey, field string, val float64) {
				nodeID := key.Tags.Get("node_id")
				collected[nodeID] = val
			})

			// Should only include US nodes (node1 and node2)
			assert.Len(t, collected, 2)
			assert.Equal(t, 0.95, collected[hex.EncodeToString(node1.Bytes())])
			assert.Equal(t, 0.85, collected[hex.EncodeToString(node2.Bytes())])
			// node3 should not be included as it's from Germany
			_, exists := collected[hex.EncodeToString(node3.Bytes())]
			assert.False(t, exists)
		},
	)
}

// mockOverlayDB implements overlay.DB for testing.
type mockOverlayDB struct {
	overlay.DB
	nodes []nodeselection.SelectedNode
	err   error
}

func (m *mockOverlayDB) GetAllParticipatingNodes(ctx context.Context, cutoff time.Duration, asOf time.Duration) ([]nodeselection.SelectedNode, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.nodes, nil
}

// mockSuccessTracker implements metainfo.SuccessTracker for testing.
type mockSuccessTracker struct {
	scores map[storj.NodeID]float64
}

func newMockSuccessTracker() *mockSuccessTracker {
	return &mockSuccessTracker{
		scores: make(map[storj.NodeID]float64),
	}
}

func (m *mockSuccessTracker) Increment(node storj.NodeID, success bool) {
	if success {
		m.scores[node]++
	}
}

func (m *mockSuccessTracker) Get(node *nodeselection.SelectedNode) float64 {
	return m.scores[node.ID]
}

func (m *mockSuccessTracker) Range(fn func(storj.NodeID, float64)) {
	for id, score := range m.scores {
		fn(id, score)
	}
}

func (m *mockSuccessTracker) BumpGeneration() {}

func (m *mockSuccessTracker) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {}
