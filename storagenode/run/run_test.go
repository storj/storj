// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/cli"
	"storj.io/storj/shared/mud"
)

func TestRunModules(t *testing.T) {
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

	s := Run{}

	selector := s.GetSelector(ball)

	result := mud.FindSelectedWithDependencies(ball, selector)

	require.True(t, len(result) > 0)
}
