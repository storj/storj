// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/private/mud"
)

// newExecCmd creates a new components command with subcommands.
func newComponentCmd(ctx context.Context, ball *mud.Ball, selector mud.ComponentSelector) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "components",
		Aliases: []string{"mud", "component"},
		Short:   "list activated / available components",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "all",
		Short: "list the name of all the defined, registered components",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdComponentAll(ball)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "list the name of activated components, and dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdComponentList(ball, selector)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "graph <basename>",
		Args:  cobra.ExactArgs(1),
		Short: "Generate SVG graph of all components. (requires dot binary of graphviz)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdComponentGraph(ball, args[0], selector)
		},
	})
	return cmd
}

func cmdComponentGraph(ball *mud.Ball, output string, selector mud.ComponentSelector) error {
	var components []*mud.Component
	err := mud.ForEachDependency(ball, selector, func(component *mud.Component) error {
		components = append(components, component)
		return nil
	}, mud.All)
	if err != nil {
		return errs.Wrap(err)
	}

	dotFileName := output + ".dot"
	dotOutput, err := os.Create(dotFileName)
	if err != nil {
		return errs.Wrap(err)
	}
	err = mud.Dot(dotOutput, components)
	if err != nil {
		return errs.Combine(err, dotOutput.Close())
	}

	err = dotOutput.Close()
	if err != nil {
		return errs.Wrap(err)
	}

	out, err := exec.Command("dot", "-Tsvg", dotFileName, "-o", output).CombinedOutput()
	if err != nil {
		return errs.New("Execution of dot is failed with %s, %v", out, err)
	}

	return nil
}

func cmdComponentAll(ball *mud.Ball) error {
	for _, c := range mud.Find(ball, mud.All) {
		fmt.Println(c.Name())
	}
	return nil
}

func cmdComponentList(ball *mud.Ball, selector mud.ComponentSelector) error {
	return mud.ForEachDependency(ball, selector, func(component *mud.Component) error {
		fmt.Println(component.Name())
		return nil
	}, mud.All)
}
