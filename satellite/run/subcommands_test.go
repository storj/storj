// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/cli"
	"storj.io/storj/shared/mud"
)

// The CLI asks every subcommand for its selector, on one ball, before it parses
// a single flag. A subcommand which registers a component another one registers
// too takes the whole binary down with it, whatever the user asked for.
func TestSubcommandSelectors(t *testing.T) {
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

	selectorOverride := reflect.TypeFor[cli.SelectorOverride]()
	var subcommands int
	require.NoError(t, mud.ForEach(ball, func(component *mud.Component) error {
		if _, found := mud.GetTagOf[cli.Subcommand](component); !found {
			return nil
		}
		if !component.GetTarget().Implements(selectorOverride) {
			return nil
		}
		require.NoError(t, component.Init(context.Background()))
		require.NotPanics(t, func() {
			component.Instance().(cli.SelectorOverride).GetSelector(ball)
		}, "%s registers a component that another subcommand registers as well", component.Name())
		subcommands++
		return nil
	}))
	require.NotZero(t, subcommands)
}
