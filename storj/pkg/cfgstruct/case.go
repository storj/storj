// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cfgstruct

import (
	"strings"
	"unicode"
)

func hyphenate(val string) string {
	return strings.Replace(val, "_", "-", -1)
}

func snakeCase(val string) string {
	// don't you think this function should be in the standard library?
	// seems useful
	if len(val) <= 1 {
		return strings.ToLower(val)
	}
	runes := []rune(val)
	rv := make([]rune, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		rv = append(rv, unicode.ToLower(runes[i]))
		if i < len(runes)-1 &&
			unicode.IsLower(runes[i]) &&
			unicode.IsUpper(runes[i+1]) {
			// lower-to-uppercase case
			rv = append(rv, '_')
		} else if i < len(runes)-2 &&
			unicode.IsUpper(runes[i]) &&
			unicode.IsUpper(runes[i+1]) &&
			unicode.IsLower(runes[i+2]) {
			// end-of-acronym case
			rv = append(rv, '_')
		}
	}
	return string(rv)
}
