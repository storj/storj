// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package mailservice

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDict(t *testing.T) {
	values, err := templateDict("Text", "Reset Password", "Simulate", true)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"Text": "Reset Password", "Simulate": true}, values)
	values, err = templateDict()
	require.NoError(t, err)
	require.Empty(t, values)
	for _, args := range [][]any{{"Text"}, {1, "value"}} {
		_, err = templateDict(args...)
		require.Error(t, err)
	}
}
