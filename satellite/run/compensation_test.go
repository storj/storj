// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/cli"
	"storj.io/storj/shared/mud"
)

// Smoketest to check if the compensation subcommands are registered with all
// their dependencies.
func TestCompensation(t *testing.T) {
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

	for _, selector := range []mud.ComponentSelector{
		mud.Select[*GenerateInvoices](ball),
		mud.Select[*RecordPeriod](ball),
		mud.Select[*RecordOneOffPayments](ball),
	} {
		result := mud.FindSelectedWithDependencies(ball, selector)
		require.True(t, len(result) > 0)
	}
}
