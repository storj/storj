// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package emission

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnit_String(t *testing.T) {
	cases := []struct {
		unit     Unit
		expected string
	}{
		{unit: Unit{}, expected: ""},
		{unit: Unit{byte: 1}, expected: "B"},
		{unit: Unit{watt: 1}, expected: "W"},
		{unit: Unit{hour: 1}, expected: "H"},
		{unit: Unit{hour: -1}, expected: "1/H"},
		{unit: Unit{kilogram: 1}, expected: "kg"},
		{unit: Unit{kilogram: -1}, expected: "1/kg"},
		{unit: Unit{byte: 1, watt: 1}, expected: "BW"},
		{unit: Unit{byte: 1, watt: -1}, expected: "B/W"},
		{unit: Unit{byte: 1, watt: -1, hour: -1}, expected: "B/WH"},
		{unit: Unit{byte: 1, kilogram: 1, watt: -1, hour: -1}, expected: "Bkg/WH"},
		{unit: Unit{byte: 2, kilogram: 1, watt: -2, hour: -1}, expected: "B²kg/W²H"},
		{unit: Unit{byte: 3, watt: -1, hour: -2}, expected: "B³/WH²"},
		{unit: Unit{byte: 2, watt: -4, hour: -1}, expected: "B²/W^4H"},
	}

	for _, c := range cases {
		t.Run("expected:"+c.expected, func(t *testing.T) {
			require.Equal(t, c.unit.String(), c.expected)
		})
	}
}
