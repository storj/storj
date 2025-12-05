// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package modular

import (
	"fmt"
	"os"
	"strings"

	"storj.io/storj/shared/mud"
)

// CreateSelector create a custom component hierarchy selector based on environment variables.
// This is the way how it is possible to replace components of the process or disable existing ones.
func CreateSelector(ball *mud.Ball) mud.ComponentSelector {
	return CreateSelectorFromString(ball, os.Getenv("STORJ_COMPONENTS"))
}

// CreateSelectorFromString creates a custom component hierarchy selector based on the provided string + adjust module hierarchy.
// selection should be a coma separated list of the following:
// * simple component name (like: debug.Wrapper) to include it with all the dependencies
// * component name with - prefix to exclude it from previous selection
// * interface=implementation to choose from already registered implementations
// * !component to disable interface (inject nil instead of any implementation).
func CreateSelectorFromString(ball *mud.Ball, selection string) mud.ComponentSelector {
	if selection == "" {
		return mud.Tagged[Service]()
	}
	var selector mud.ComponentSelector = func(c *mud.Component) bool {
		return false
	}

	for _, s := range strings.Split(selection, ",") {
		switch {
		case s == "service":
			selector = mud.Or(selector, mud.Tagged[Service]())
		case strings.HasPrefix(s, "-"):
			selector = mud.And(selector, excludeType(s[1:]))
		case strings.HasPrefix(s, "~"):
			c := mud.Find(ball, includeType(s[1:]))
			if len(c) != 1 {
				panic(fmt.Sprintf("component selector %s should match exactly one component", s[1:]))
			}
			mud.AddTagOf[mud.Optional](c[0], mud.Optional{})
		case strings.HasPrefix(s, "$"):
			c := mud.Find(ball, includeType(s[1:]))
			if len(c) != 1 {
				panic(fmt.Sprintf("component selector %s should match exactly one component", s[1:]))
			}
			mud.RemoveTagOf[mud.Optional](c[0])
		case strings.HasPrefix(s, "!"):
			interf := mud.Find(ball, includeType(s[1:]))
			if len(interf) != 1 {
				panic(fmt.Sprintf("interface selector %s should match exactly one component", s[1:]))
			}
			mud.DisableImplementationOf(interf[0])
		case strings.Contains(s, "="):
			interf, impl, _ := strings.Cut(s, "=")
			from := mud.Find(ball, includeType(interf))
			if len(from) != 1 {
				panic(fmt.Sprintf("interface selector %s should match exactly one component", interf))
			}
			to := mud.Find(ball, includeType(impl))
			if len(to) != 1 {
				panic(fmt.Sprintf("implementation selector %s should match exactly one component", impl))
			}
			mud.ReplaceDependencyOf(from[0], to[0])
		default:
			to := mud.Find(ball, includeType(s))
			if len(to) != 1 {
				panic(fmt.Sprintf("implementation selector %s should match one component", s))
			}
			selector = mud.Or(selector, includeType(s))
		}
	}
	return selector
}

func includeType(name string) mud.ComponentSelector {
	return func(c *mud.Component) bool {
		componentName := c.GetTarget().String()
		return componentName == name || componentName == "*"+name
	}
}

func excludeType(name string) mud.ComponentSelector {
	return func(c *mud.Component) bool {
		componentName := c.GetTarget().String()
		return componentName != name && componentName != "*"+name
	}
}
