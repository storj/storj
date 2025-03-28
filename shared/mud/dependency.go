// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

import (
	"fmt"
	"reflect"
)

// Optional tag is used to mark components which may not required.
type Optional struct{}

// Find selects components matching the selector.
func Find(ball *Ball, selector ComponentSelector) (result []*Component) {
	for _, c := range ball.registry {
		if selector(c) {
			result = append(result, c)
		}
	}
	return result
}

// FindSelectedWithDependencies selects components matching the selector, together with all the dependencies.
func FindSelectedWithDependencies(ball *Ball, selector ComponentSelector) (result []*Component) {
	dependencies := map[reflect.Type]struct{}{}
	for _, component := range ball.registry {
		if selector(component) {
			collectDependencies(ball, component, dependencies)
		}
	}
	for k := range dependencies {
		result = append(result, mustLookupByType(ball, k))
	}
	return filterComponents(sortedComponents(ball), result)
}

func collectDependencies(ball *Ball, c *Component, result map[reflect.Type]struct{}) {
	// don't check it again
	for k := range result {
		if c.target == k {
			return
		}
	}

	// don't check it again
	result[c.target] = struct{}{}

	for _, dep := range c.requirements {
		// ignore if optional
		dc, found := LookupByType(ball, dep)
		if !found {
			panic(fmt.Sprintf("Dependency %s for %s is missing", dep, c.ID()))
		}
		_, optional := findTag[Optional](dc)
		if optional {
			continue
		}
		collectDependencies(ball, mustLookupByType(ball, dep), result)
	}
}
