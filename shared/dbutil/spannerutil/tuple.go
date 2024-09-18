// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"strings"

	"github.com/zeebo/errs"
)

// TupleGreaterThanSQL returns a constructed SQL expression equivalent to a
// tuple comparison (e.g. (tup1[0], tup1[1], ...) > (tup2[0], tup2[1], ...)).
//
// If orEqual is true, the returned expression will compare the tuples as
// "greater than or equal" (>=) instead of "greater than" (>).
//
// This is necessary because Spanner does not support comparison of tuples,
// except with equality (=).
//
// Example:
//
//	(a, b, c) >= (d, e, f)
//
// becomes
//
//	TupleGreaterThanSQL([]string{"a", "b", "c"}, []string{"d", "e", "f"}, true)
//
// which returns
//
//	"((a > d) OR (a = d AND b > e) OR (a = d AND b = e AND c >= f))"
func TupleGreaterThanSQL(tup1, tup2 []string, orEqual bool) (string, error) {
	if len(tup1) != len(tup2) {
		return "", errs.New("programming error: comparing tuples of different lengths")
	}
	if len(tup1) == 0 {
		return "", errs.New("programming error: comparing tuples of zero length")
	}
	comparator := " > "
	if orEqual {
		comparator = " >= "
	}
	var sb strings.Builder
	if len(tup1) > 1 {
		sb.WriteString("(")
	}
	for i := range tup1 {
		sb.WriteString("(")
		for j := 0; j < i; j++ {
			sb.WriteString(tup1[j])
			sb.WriteString(" = ")
			sb.WriteString(tup2[j])
			sb.WriteString(" AND ")
		}
		sb.WriteString(tup1[i])
		if i == len(tup1)-1 {
			sb.WriteString(comparator)
		} else {
			sb.WriteString(" > ")
		}
		sb.WriteString(tup2[i])
		sb.WriteString(")")
		if i < len(tup1)-1 {
			sb.WriteString(" OR ")
		}
	}
	if len(tup1) > 1 {
		sb.WriteString(")")
	}
	return sb.String(), nil
}
