// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import "strings"

// Placeholders returns n comma separated "?" placeholders, for example to build
// an "IN (...)" list which works on all supported databases. The query needs to
// be passed through Rebind. n must be greater than zero.
func Placeholders(n int) string {
	return strings.Repeat("?, ", n-1) + "?"
}

// InArgs converts values into query arguments matching Placeholders(len(values)).
func InArgs[T any](values []T) []any {
	args := make([]any, len(values))
	for i, v := range values {
		args[i] = v
	}
	return args
}
