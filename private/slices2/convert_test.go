// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package slices2_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/private/slices2"
)

func TestConvert(t *testing.T) {
	good := []string{"1", "2", "3", "4"}
	out, err := slices2.Convert(good, strconv.Atoi)
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3, 4}, out)

	bad := []string{"1", "bad", "asdf", ""}
	out, err = slices2.Convert(bad, strconv.Atoi)
	require.Error(t, err)
	require.Nil(t, out)
}
