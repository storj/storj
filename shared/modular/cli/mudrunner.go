// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/zeebo/clingy"

	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Run is a generic helper to run a modular application.
// Includes common subcommands like `exec` and any other component which is registered with RegisterSubcommand.
func Run(module func(ball *mud.Ball)) {
	ctx, cancel := context.WithCancel(context.Background())
	ctx, _ = signal.NotifyContext(ctx, os.Interrupt)

	ball := mud.NewBall()
	{
		// register generic subcommands
		mud.Provide[*Version](ball, NewVersion)
		RegisterSubcommand[*Version](ball, "version", "print version information")

		mud.Provide[*ComponentList](ball, NewComponentList)
		RegisterSubcommand[*ComponentList](ball, "components-list", "list the name of activated components, and dependencies")
		config.RegisterConfig[ComponentListConfig](ball, "")

		mud.Provide[*ComponentAll](ball, NewComponentAll)
		RegisterSubcommand[*ComponentAll](ball, "components-all", "list the name of all the defined, registered components")

		mud.Provide[*ComponentGraph](ball, NewComponentGraph)
		RegisterSubcommand[*ComponentGraph](ball, "components-graph", "generate SVG graph of all components. (requires dot binary of graphviz)")
		config.RegisterConfig[ComponentGraphConfig](ball, "")
	}

	module(ball)
	mud.Supply[*modular.StopTrigger](ball, &modular.StopTrigger{
		Cancel: cancel,
	})

	cfg := &ConfigSupport{}
	ok, err := clingy.Environment{
		Dynamic: cfg.GetValue,
	}.Run(ctx, clingyRunner(cfg, ball))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
	if !ok || err != nil {
		os.Exit(1)
	}
}

func clingyRunner(cfg *ConfigSupport, ball *mud.Ball) func(cmds clingy.Commands) {
	return func(cmds clingy.Commands) {

		// register clingy parameters
		cfg.Setup(cmds)

		// standard exec subcommand
		cmds.New("exec", "run services (or just the selected components)", &MudCommand{
			ball: ball,
			cfg:  cfg,
		})

		// add all registered subcommands
		err := mud.ForEach(ball, func(component *mud.Component) error {
			sc, found := mud.GetTagOf[Subcommand](component)
			if !found {
				return nil
			}
			cmdSelector := func(c *mud.Component) bool {
				return c == component
			}
			cmds.New(sc.Name, sc.Description, &MudCommand{
				ball:     ball,
				selector: cmdSelector,
				cfg:      cfg,
			})
			return nil
		})
		if err != nil {
			panic(err)
		}
	}
}
