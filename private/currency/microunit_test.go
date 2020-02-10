// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package currency

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMicroUnitToFloatString(t *testing.T) {
	require.Equal(t, "1.002332", MicroUnit(1002332).FloatString())
}

func TestMicroUnitFromFloatString(t *testing.T) {
	m, err := MicroUnitFromFloatString("0.012340")
	require.NoError(t, err)
	require.Equal(t, MicroUnit(12340), m)
}
