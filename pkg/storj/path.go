// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"path"
	"strings"
)

// Path represents a object path
type Path = string

// PathComponents splits the p path into a slice of path components
func PathComponents(p Path) []string {
	if p == "" {
		return []string{}
	}

	p = path.Clean(p)
	p = strings.Trim(p, "/")
	comps := strings.Split(p, "/")

	if len(comps) == 1 && comps[0] == "" {
		return []string{}
	}

	return comps
}

// TrimLeftPathComponents returns p without the leading num components
func TrimLeftPathComponents(p Path, num int) Path {
	comps := PathComponents(p)
	if num > len(comps) {
		return ""
	}
	return path.Join(comps[num:]...)
}
