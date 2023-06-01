// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertSlice(t *testing.T) {
	good := []string{"1", "2", "3", "4"}
	out, err := convertSlice(good, strconv.Atoi)
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3, 4}, out)

	bad := []string{"1", "bad", "asdf", ""}
	out, err = convertSlice(bad, strconv.Atoi)
	require.Error(t, err)
	require.Nil(t, out)
}
