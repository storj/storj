// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package currency

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/strictcsv"
)

func TestMicroUnitToFloatString(t *testing.T) {
	require.Equal(t, "1.002332", NewMicroUnit(1002332).FloatString())
}

func TestMicroUnitFromFloatString(t *testing.T) {
	m, err := MicroUnitFromFloatString("0.012340")
	require.NoError(t, err)
	require.Equal(t, NewMicroUnit(12340), m)
}

func TestMicroUnitCSV(t *testing.T) {
	type row struct {
		Foo MicroUnit `csv:"foo"`
		Bar MicroUnit `csv:"bar"`
	}
	exp := row{
		Foo: NewMicroUnit(1),
		Bar: NewMicroUnit(2),
	}

	csv, err := strictcsv.MarshalString(exp)
	require.NoError(t, err)

	var got row
	require.NoError(t, strictcsv.UnmarshalString(csv, &got))
	require.Equal(t, exp, got)
}
