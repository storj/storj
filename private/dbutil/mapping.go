// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"fmt"
	"strings"
)

// EscapableCommaSplit is like strings.Split(x, ","), but if
// it sees two ','s in a row, it will treat them like one
// unsplit comma. So "hello,there,,friend" will result in
// ["hello", "there,friend"].
func EscapableCommaSplit(val string) []string {
	bytes := []byte(val)
	var vals []string
	current := make([]byte, 0, len(bytes))
	for i := 0; i < len(bytes); i++ {
		char := bytes[i]
		if char == ',' {
			if i < len(bytes)-1 && bytes[i+1] == ',' {
				current = append(current, ',')
				i++
			} else {
				vals = append(vals, string(current))
				current = nil
			}
		} else {
			current = append(current, char)
		}
	}
	vals = append(vals, string(current))
	return vals
}

// ParseDBMapping parses a mapping of database connection URLs, preceded
// by the default URL. An example that overrides the repairqueue looks like:
// cockroach://user:pw@host/database,repairqueue:postgres://user:pw@host/database.
// The default is stored in "".
func ParseDBMapping(urlSpec string) (map[string]string, error) {
	parts := EscapableCommaSplit(urlSpec)
	rv := map[string]string{
		"": parts[0],
	}
	for _, other := range parts[1:] {
		override := strings.SplitN(other, ":", 2)
		if len(override) != 2 || strings.HasPrefix(override[1], "/") {
			return nil, fmt.Errorf("invalid db mapping spec: %q", urlSpec)
		}
		rv[override[0]] = override[1]
	}
	return rv, nil
}
