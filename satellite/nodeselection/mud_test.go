// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/shared/mud"
)

type constantTracker float64

func (c constantTracker) Get(uplink storj.NodeID) func(node *nodeselection.SelectedNode) float64 {
	return func(node *nodeselection.SelectedNode) float64 { return float64(c) }
}

func TestModulePlacementTrackers(t *testing.T) {
	ctx := testcontext.New(t)

	initEnv := func(t *testing.T, selectTrackers bool) nodeselection.PlacementConfigEnvironment {
		ball := mud.NewBall()
		mud.Provide[*nodeselection.PlacementTrackers](ball, func() *nodeselection.PlacementTrackers {
			return &nodeselection.PlacementTrackers{Success: constantTracker(42)}
		})
		mud.Tag[*nodeselection.PlacementTrackers](ball, mud.Optional{})
		mud.Tag[*nodeselection.PlacementTrackers](ball, mud.Nullable{})
		mud.Supply[nodeselection.PlacementDefinitions](ball, nodeselection.PlacementDefinitions{})
		nodeselection.Module(ball)

		selector := mud.Select[nodeselection.PlacementConfigEnvironment](ball)
		if selectTrackers {
			selector = mud.Or(selector, mud.Select[*nodeselection.PlacementTrackers](ball))
		}
		require.NoError(t, mud.ForEachDependency(ball, selector, mud.Initialize(ctx), mud.All))
		return mud.MustLookup[nodeselection.PlacementConfigEnvironment](ball)
	}

	t.Run("selected trackers are used", func(t *testing.T) {
		env := initEnv(t, true)
		require.Equal(t, constantTracker(42), env["uploadSuccessTracker"])
	})

	t.Run("noop trackers without selection", func(t *testing.T) {
		env := initEnv(t, false)
		require.Equal(t, nodeselection.NoopSuccessTracker{}, env["uploadSuccessTracker"])
	})
}
