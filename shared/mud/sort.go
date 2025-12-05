// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

import (
	"reflect"
)

// sortedComponents returns components in order to start/run/close them.
// it implements a simple topology sorting based on Kahn's algorithm:  https://en.wikipedia.org/wiki/Topological_sorting
func sortedComponents(ball *Ball) (sorted []*Component) {
	// key should be initialized before the values
	dependencyGraph := make(map[reflect.Type][]reflect.Type)

	for _, component := range ball.registry {
		if _, found := dependencyGraph[component.target]; !found {
			dependencyGraph[component.target] = make([]reflect.Type, 0)
		}

		dependencyGraph[component.target] = append(dependencyGraph[component.target], component.requirements...)
	}

	var next []reflect.Type

	findNext := func() {
		filtered := map[reflect.Type][]reflect.Type{}
		for c, deps := range dependencyGraph {
			if len(deps) == 0 {
				next = append(next, c)
			} else {
				filtered[c] = deps
			}
		}
		dependencyGraph = filtered
	}

	without := func(deps []reflect.Type, s reflect.Type) (res []reflect.Type) {
		for _, d := range deps {
			if d != s {
				res = append(res, d)
			}
		}
		return res
	}

	findNext()

	for len(next) > 0 {
		s := next[0]
		next = next[1:]

		component, found := LookupByType(ball, s)
		if !found {
			panic("component is not registered " + s.String())
		}
		sorted = append(sorted, component)

		for c, deps := range dependencyGraph {
			dependencyGraph[c] = without(deps, s)
		}

		findNext()
	}
	if len(dependencyGraph) > 0 {
		problems := "   "
		for c, deps := range dependencyGraph {
			problems += c.String()
			problems += " > "
			for _, dep := range deps {
				found := false
				for _, s := range sorted {
					if s.target == dep {
						found = true
						break
					}
				}
				if !found {
					problems += dep.String() + " "
				}
			}
			problems += ";\n    "
		}

		panic("Unresolved dependencies:\n " + problems)
	}
	return sorted
}

func filterComponents(sorted []*Component, required []*Component) (result []*Component) {
	for _, s := range sorted {
		for _, r := range required {
			if r.target == s.target {
				result = append(result, r)
				break
			}
		}
	}
	return result
}
