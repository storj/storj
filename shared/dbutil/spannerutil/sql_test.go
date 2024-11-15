// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitSQLStatements(t *testing.T) {
	t.Parallel()

	sql := `ABC; DEF; GHI;
JKL: MNO; "PQR; STU"; VWX
'y\';z"'; "a\";b\\\'"; c
-- comment
d;

e;f;g/*;;*/h; ;i
`

	stmts, err := SplitSQLStatements(sql)
	require.NoError(t, err)

	require.Equal(t, []string{
		"ABC",
		" DEF",
		" GHI",
		"\nJKL: MNO",
		" \"PQR; STU\"",
		" VWX\n'y\\';z\"'",
		" \"a\\\";b\\\\\\'\"",
		" c\n\nd",
		"\n\ne",
		"f",
		"gh",
		"i\n",
	}, stmts)

	sql2 := "foo;`bar\\`;/*baz`;boom*/;"

	stmts2, err := SplitSQLStatements(sql2)
	require.NoError(t, err)

	require.Equal(t, []string{
		"foo",
		"`bar\\`;/*baz`",
		"boom*/",
	}, stmts2)
}
