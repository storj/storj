// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTupleGreaterThanSQLText(t *testing.T) {
	result, err := TupleGreaterThanSQL([]string{"a", "b", "c"}, []string{"d", "e", "f"}, true)
	require.NoError(t, err)
	assert.Equal(t, "((a > d) OR (a = d AND b > e) OR (a = d AND b = e AND c >= f))", result)

	result, err = TupleGreaterThanSQL([]string{"a", "b", "c"}, []string{"d", "e", "f"}, false)
	require.NoError(t, err)
	assert.Equal(t, "((a > d) OR (a = d AND b > e) OR (a = d AND b = e AND c > f))", result)

	result, err = TupleGreaterThanSQL([]string{"a"}, []string{"b"}, true)
	require.NoError(t, err)
	assert.Equal(t, "(a >= b)", result)

	result, err = TupleGreaterThanSQL([]string{"a"}, []string{"b"}, false)
	require.NoError(t, err)
	assert.Equal(t, "(a > b)", result)
}
