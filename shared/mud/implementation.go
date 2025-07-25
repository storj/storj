// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

import (
	"context"
	"slices"
)

// RegisterImplementation registers the implementation interface, without adding concrete implementation.
func RegisterImplementation[L ~[]T, T any](ball *Ball) {
	RegisterManual[L](ball, func(ctx context.Context) (L, error) {
		var instances L
		component := lookup[L](ball)
		for _, req := range component.requirements {
			c, _ := LookupByType(ball, req)
			// only initialized instances are inject to the implementation list
			if c.instance != nil {
				instances = append(instances, c.instance.(T))
			}
		}
		return instances, nil
	})
}

// Implementation registers a new []T component, which will be filled with any registered instances.
// Instances will be marked with "Optional{}" tag, and will be injected only, if they are initialized.
// It's the responsibility of the Init code to exclude / include them during initialization.
func Implementation[L ~[]T, Instance any, T any](ball *Ball) {
	if lookup[L](ball) == nil {
		RegisterImplementation[L, T](ball)
	}
	lookup[L](ball).requirements = append(lookup[L](ball).requirements, typeOf[Instance]())
}

// ImplementationOf is a ForEach filter to get all the dependency of an implementation.
func ImplementationOf[L ~[]T, T any](ball *Ball) ComponentSelector {
	component := MustLookupComponent[L](ball)
	return func(c *Component) bool {
		return slices.Contains(component.requirements, c.target)
	}
}
