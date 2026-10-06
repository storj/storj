// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"slices"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"storj.io/common/cfgstruct"
	"storj.io/common/debug"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/cli"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Smoketest to check if all required modules are registered properly.
func TestApi(t *testing.T) {
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

	s := Api{}

	selector := s.GetSelector(ball)

	result := mud.FindSelectedWithDependencies(ball, selector)

	require.True(t, len(result) > 0)
	require.True(t, slices.ContainsFunc(result, mud.SelectIfExists[*metainfo.TrackerInfo]()), "api should serve the /trackers debug page")

	// Phase 1 of the real startup (see cli.MudCommand.Execute): only RunEarly
	// components and their dependencies are initialized before the debug server.
	ctx := testcontext.New(t)
	runEarly := mud.And(mud.DependencyOf(ball, selector), mud.Tagged[modular.RunEarly]())
	require.NoError(t, config.BindAll(ctx, &cobra.Command{}, ball, runEarly, cfgstruct.UseTestDefaults()))
	// the extension list is built even without a listener.
	mud.MustLookup[*debug.Config](ball).Addr = ""
	require.NoError(t, mud.ForEachDependency(ball, runEarly, mud.Initialize(ctx), mud.All))
	defer func() {
		require.NoError(t, mud.ForEachDependencyReverse(ball, runEarly, func(c *mud.Component) error {
			return c.Close(ctx)
		}, mud.All))
	}()

	extensions := mud.MustLookup[[]debug.Extension](ball)
	require.True(t, slices.ContainsFunc(extensions, func(e debug.Extension) bool {
		_, ok := e.(*metainfo.TrackerInfoExtension)
		return ok
	}), "/trackers should be registered before the debug server starts")

	core := mud.FindSelectedWithDependencies(ball, (&Core{}).GetSelector(ball))
	require.False(t, slices.ContainsFunc(core, mud.SelectIfExists[*metainfo.TrackerInfo]()), "only api has the success trackers")
}
