// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"syscall"

	"github.com/zeebo/clingy"

	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Run is a generic helper to run a modular application.
// Includes common subcommands like `exec` and any other component which is registered with RegisterSubcommand.
func Run(module func(ball *mud.Ball)) {
	ctx, cancel := context.WithCancel(context.Background())
	var stop context.CancelFunc
	ctx, stop = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

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

		mud.Provide[*ConfigList](ball, NewConfigList)
	}

	module(ball)
	mud.Supply[*modular.StopTrigger](ball, &modular.StopTrigger{
		Cancel: cancel,
	})

	cfg := &ConfigSupport{}
	mud.Supply[*ConfigSupport](ball, cfg)
	mud.View[*ConfigSupport, ConfigDir](ball, func(support *ConfigSupport) ConfigDir {
		return ConfigDir{
			Dir: support.configDir,
		}
	})
	ok, err := clingy.Environment{
		Dynamic: cfg.GetValue,
	}.Run(ctx, clingyRunner(cfg, ball))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
	if !ok || err != nil {
		stop()
		//nolint:gocritic // stop() is called explicitly
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

		cmds.New("config-list", "List available config options and actual values", &MudCommand{
			ball:        ball,
			runSelector: mud.Select[*ConfigList](ball),
			cfg:         cfg,
		})

		// collect all registered subcommands, keeping top-level ones separate
		// from those nested under a group so that grouped commands can be emitted
		// together with a single clingy.Group call.
		type subcommand struct {
			sc  Subcommand
			cmd *MudCommand
		}
		var topLevel []subcommand
		grouped := map[string][]subcommand{}
		var groupOrder []string
		groupDescs := map[string]string{}

		err := mud.ForEach(ball, func(component *mud.Component) error {
			sc, found := mud.GetTagOf[Subcommand](component)
			if !found {
				return nil
			}
			cmdSelector := func(c *mud.Component) bool {
				return c == component
			}

			// A specific case when the component implements SelectorOverride interface.
			// In this case we call it for the real components to run, instead of just using the Run (what we usually do for tool subcommands).
			selectorOverrideType := reflect.TypeFor[SelectorOverride]()
			if component.GetTarget().Implements(selectorOverrideType) {
				err := component.Init(context.Background())
				if err != nil {
					panic(err)
				}
				cmdSelector = component.Instance().(SelectorOverride).GetSelector(ball)
			}

			entry := subcommand{
				sc: sc,
				cmd: &MudCommand{
					ball:     ball,
					selector: cmdSelector,
					cfg:      cfg,
				},
			}

			if sc.Group == "" {
				topLevel = append(topLevel, entry)
				return nil
			}
			if _, ok := grouped[sc.Group]; !ok {
				groupOrder = append(groupOrder, sc.Group)
			}
			if groupDescs[sc.Group] == "" {
				groupDescs[sc.Group] = sc.GroupDescription
			}
			grouped[sc.Group] = append(grouped[sc.Group], entry)
			return nil
		})
		if err != nil {
			panic(err)
		}

		for _, entry := range topLevel {
			cmds.New(entry.sc.Name, entry.sc.Description, entry.cmd)
		}
		for _, group := range groupOrder {
			children := grouped[group]
			cmds.Group(group, groupDescs[group], func() {
				for _, entry := range children {
					cmds.New(entry.sc.Name, entry.sc.Description, entry.cmd)
				}
			})
		}
	}
}

// SelectorOverride is an interface for components that can override the selector used in the command (instead of executing Run).
type SelectorOverride interface {
	GetSelector(ball *mud.Ball) mud.ComponentSelector
}
