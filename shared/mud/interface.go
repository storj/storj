// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

import (
	"context"
	"fmt"
	"reflect"
)

// Interface is a marker tag, to make it easier to list all possible extension points.
// Only used for debug / helper commands.
type Interface struct {
}

func (i Interface) String() string {
	return "interface"
}

// CustomizeDotNode customize the graphical representation of the node in the graph, when rendered for debugging.
func (i Interface) CustomizeDotNode(tags []string) []string {
	return append(tags, "fontcolor=blue")
}

// RegisterInterfaceImplementation registers an interface with an implementation. Later the implementation can be replaced.
// Only one (or zero) implementation can be registered/used at the same time.
func RegisterInterfaceImplementation[BASE any, DEP any](ball *Ball) {
	RegisterManual[BASE](ball, func(ctx context.Context) (BASE, error) {
		base := lookup[BASE](ball)
		if len(base.requirements) > 1 {
			panic(fmt.Sprintf("RegisterInterfaceImplementation should have zero or one dependency, but %v found, for %v", len(base.requirements), typeOf[BASE]()))
		}

		// the case of optional dependency
		if len(base.requirements) == 0 {
			var ret BASE
			return ret, nil
		}
		c, _ := LookupByType(ball, base.requirements[0])
		if c.instance == nil {
			panic(fmt.Sprintf("The registered depdenency is not yet initialized %v->%v", typeOf[BASE](), typeOf[DEP]()))
		}
		return c.instance.(BASE), nil
	})
	DependsOn[BASE, DEP](ball)
	Tag[BASE, Interface](ball, Interface{})
}

// DisableImplementation removes the implementation from the list of dependencies.
func DisableImplementation[BASE any](ball *Ball) {
	c := MustLookupComponent[BASE](ball)
	c.requirements = []reflect.Type{}
	AddTagOf[Nullable](c, Nullable{})
}

// DisableImplementationOf is like DisableImplementation, but using components instead of generics.
func DisableImplementationOf(c *Component) {
	c.requirements = []reflect.Type{}
	AddTagOf[Nullable](c, Nullable{})
	AddTagOf[Optional](c, Optional{})
}

// ReplaceDependency replaces the dependency of a component. Can be used to switch to an alternative implementation.
func ReplaceDependency[BASE any, DEP any](ball *Ball) {
	c := MustLookupComponent[BASE](ball)
	c.requirements = []reflect.Type{typeOf[DEP]()}
}

// ReplaceDependencyOf is like ReplaceDependency but using components instead of generics.
func ReplaceDependencyOf(from *Component, to *Component) {
	from.requirements = []reflect.Type{to.target}
}
