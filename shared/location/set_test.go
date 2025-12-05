// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package location

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {
	var set Set
	set.Include(Belgium)
	set.Include(Hungary)
	set.Include(TheNetherlands)
	set.Include(ToCountryCode("ZZ"))

	require.True(t, set.Contains(Belgium))
	require.True(t, set.Contains(Hungary))
	require.True(t, set.Contains(TheNetherlands))
	require.True(t, set.Contains(ToCountryCode("ZZ")))

	require.False(t, set.Contains(Estonia))
	require.False(t, set.Contains(Austria))

	require.Equal(t, 4, set.Count())

	set.Remove(Hungary)
	// removing non-existent things should be fine
	set.Remove(Estonia)
	set.Remove(Austria)
	require.Equal(t, 3, set.Count())
}

func TestSet_Full(t *testing.T) {
	var set Set
	for c := CountryCode(0); int(c) < len(CountryISOCode); c++ {
		set.Include(c)
	}
	require.Equal(t, len(CountryISOCode), set.Count())

	for c := CountryCode(0); int(c) < len(CountryISOCode); c++ {
		set.Remove(c)
	}
	require.Equal(t, 0, set.Count())
}
