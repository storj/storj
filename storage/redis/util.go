// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

func escapeMatch(match []byte) []byte {
	start := 0
	escaped := []byte{}
	for i, b := range match {
		switch b {
		case '?', '*', '[', ']', '\\':
			escaped = append(escaped, match[start:i]...)
			escaped = append(escaped, '\\', b)
			start = i + 1
		}
	}
	if start == 0 {
		return match
	}

	return append(escaped, match[start:]...)
}
