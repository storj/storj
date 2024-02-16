// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

// Select is a component selector based on the specified type ([A]).
func Select[A any](ball *Ball) ComponentSelector {
	t := typeOf[A]()
	mustLookup[A](ball)
	return func(c *Component) bool {
		return c.target == t
	}
}

// ComponentSelector can filter components.
type ComponentSelector func(c *Component) bool

// All is a ComponentSelector which matches all components.
func All(_ *Component) bool {
	return true
}

// And selects components which matches all the selectors.
func And(selectors ...ComponentSelector) ComponentSelector {
	return func(c *Component) bool {
		for _, s := range selectors {
			if !s(c) {
				return false
			}
		}
		return true
	}
}

// Or selects components which matches any of the selectors.
func Or(selectors ...ComponentSelector) ComponentSelector {
	return func(c *Component) bool {
		for _, s := range selectors {
			if s(c) {
				return true
			}
		}
		return false
	}
}

// Tagged is a selector, checking an annotation key/value.
func Tagged[Tag any]() func(c *Component) bool {
	return func(c *Component) bool {
		_, found := findTag[Tag](c)
		return found
	}
}
