// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package modular

import (
	"os"
	"strings"

	"storj.io/storj/private/mud"
)

// CreateSelector create a custom component hierarchy selector based on environment variables.
// This is the way how it is possible to replace components of the process or disable existing ones.
func CreateSelector() mud.ComponentSelector {
	selection := os.Getenv("STORJ_COMPONENTS")
	if selection == "" {
		return mud.Tagged[Service]()
	}
	var selectors []mud.ComponentSelector
	for _, s := range strings.Split(selection, ",") {
		if s == "service" {
			selectors = append(selectors, mud.Tagged[Service]())
		} else if strings.HasPrefix(s, "-") {
			selectors = []mud.ComponentSelector{mud.And(mud.Or(selectors...), excludeType(s[1:]))}
		} else {
			selectors = append(selectors, includeType(s))
		}
	}
	return mud.Or(selectors...)
}

func includeType(name string) mud.ComponentSelector {
	return func(c *mud.Component) bool {
		return c.GetTarget().String() == name
	}
}

func excludeType(name string) mud.ComponentSelector {
	return func(c *mud.Component) bool {
		return c.GetTarget().String() != name
	}
}
