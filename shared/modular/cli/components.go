// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/zeebo/errs"

	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/mud"
)

// ComponentList lists components based on a selector.
type ComponentList struct {
	config *ComponentListConfig
	ball   *mud.Ball
}

// ComponentListConfig contains configuration for the component list command.
type ComponentListConfig struct {
	Selector string `help:"Selector for components to include in the graph" default:""`
}

// NewComponentList creates a new ComponentList command.
func NewComponentList(ball *mud.Ball, cfg *ComponentListConfig) *ComponentList {
	return &ComponentList{
		config: cfg,
		ball:   ball,
	}
}

// Run executes the component list command, printing matching component names.
func (c *ComponentList) Run() error {
	selector := modular.CreateSelectorFromString(c.ball, c.config.Selector)
	return mud.ForEachDependency(c.ball, selector, func(component *mud.Component) error {
		fmt.Println(component.Name())
		return nil
	}, mud.All)
}

// ComponentAll lists all components in the dependency graph.
type ComponentAll struct {
	ball *mud.Ball
}

// NewComponentAll creates a new ComponentAll command.
func NewComponentAll(ball *mud.Ball) *ComponentAll {
	return &ComponentAll{
		ball: ball,
	}
}

// Run executes the component all command, printing all component names.
func (c *ComponentAll) Run() error {
	for _, comp := range mud.Find(c.ball, mud.All) {
		fmt.Println(comp.Name())
	}
	return nil
}

// ComponentGraphConfig contains configuration for the component graph command.
type ComponentGraphConfig struct {
	Selector string `help:"Selector for components to include in the graph" default:"all"`
	Output   string `help:"Output file for the graph (without extension)" default:"compomnents"`
}

// ComponentGraph generates a graph visualization of components.
type ComponentGraph struct {
	ball   *mud.Ball
	config *ComponentGraphConfig
}

// NewComponentGraph creates a new ComponentGraph command.
func NewComponentGraph(ball *mud.Ball, cfg *ComponentGraphConfig) *ComponentGraph {
	return &ComponentGraph{
		ball:   ball,
		config: cfg,
	}
}

// Run executes the component graph command, generating DOT and SVG files.
func (c *ComponentGraph) Run() error {
	var components []*mud.Component
	selector := modular.CreateSelectorFromString(c.ball, c.config.Selector)
	err := mud.ForEachDependency(c.ball, selector, func(component *mud.Component) error {
		components = append(components, component)
		return nil
	}, mud.All)
	if err != nil {
		return errs.Wrap(err)
	}

	dotFileName := c.config.Output + ".dot"
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

	out, err := exec.Command("dot", "-Tsvg", dotFileName, "-o", c.config.Output+".svg").CombinedOutput()
	if err != nil {
		return errs.New("Execution of dot is failed with %s, %v", out, err)
	}

	return nil
}
