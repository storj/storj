// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/cli"
	"storj.io/storj/shared/mud"
)

// The shards of a cycle would be spread over separate iterations.
func TestGcBfCycleCheck(t *testing.T) {
	_, err := NewGcBfCycleCheck(bloomfilter.Config{ShardCount: 1})
	require.NoError(t, err)

	_, err = NewGcBfCycleCheck(bloomfilter.Config{ShardCount: 3})
	require.Error(t, err)
}

// The once job scans a snapshot it pins itself, and must not have the periodic
// loop service running next to it over the same observers.
func TestGcBfOnceHasNoPeriodicLoop(t *testing.T) {
	ball := mud.NewBall()

	// these are provided by the CLI environment
	mud.Provide[*modular.StopTrigger](ball, func() *modular.StopTrigger {
		return &modular.StopTrigger{}
	})
	mud.Provide[*cli.ConfigDir](ball, func() *cli.ConfigDir {
		return &cli.ConfigDir{Dir: t.TempDir()}
	})
	mud.View[*cli.ConfigDir, cli.ConfigDir](ball, mud.Dereference)

	Module(ball)

	result := mud.FindSelectedWithDependencies(ball, (&GcBfOnce{}).GetSelector(ball))

	require.False(t, slices.ContainsFunc(result, func(c *mud.Component) bool {
		return c.Name() == "*rangedloop.Service"
	}), "the periodic ranged loop service would run next to the once job")

	// the observer drives the shard passes, so it has to be in the graph
	require.True(t, slices.ContainsFunc(result, func(c *mud.Component) bool {
		return c.Name() == "*bloomfilter.Observer"
	}), "the bloom filter observer is not selected")
}
