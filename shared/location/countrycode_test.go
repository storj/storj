// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package location

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCountryCode_String(t *testing.T) {
	require.Equal(t, CountryCode(countryCodeZZ), ToCountryCode("ZZ"))
	require.Equal(t, "HU", ToCountryCode("HU").String())
	require.Equal(t, "DE", ToCountryCode("DE").String())
	require.Equal(t, "XX", ToCountryCode("XX").String())
	require.Equal(t, "ZZ", ToCountryCode("ZZ").String())
	require.Equal(t, "", None.String())
}

func TestCountryCode_SQLConversion(t *testing.T) {
	p := Cyprus
	value, err := p.Value()
	require.NoError(t, err)

	res := new(CountryCode)
	err = res.Scan(value)
	require.NoError(t, err)
	require.Equal(t, Cyprus, *res)

	err = res.Scan(nil)
	require.NoError(t, err)
	require.Equal(t, None, *res)

	err = res.Scan(123)
	require.Error(t, err)
}

var sink string

var countriesForTesting = []CountryCode{
	Austria, Belgium, Bulgaria, Croatia, Cyprus, Czechia, Denmark, Estonia,
	Finland, France, Germany, Greece, Hungary, Ireland, Italy, Lithuania,
	Latvia, Luxembourg, Malta, TheNetherlands, Poland, Portugal, Romania, Slovenia,
	Slovakia, Spain, Sweden,
}

func BenchmarkCountryCode_String(b *testing.B) {
	for i := 0; i < b.N; i++ {
		code := countriesForTesting[i%len(countriesForTesting)]
		sink = code.String()
	}
}
