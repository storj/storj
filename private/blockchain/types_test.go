// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.
package blockchain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBytesToAddress(t *testing.T) {
	a := Address{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 10}
	gotA, err := BytesToAddress(a.Bytes())
	require.NoError(t, err)
	require.Equal(t, a, gotA)

	_, err = BytesToAddress([]byte{1, 2, 3})
	require.Error(t, err)

}
