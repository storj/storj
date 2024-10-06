// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/geoip"
	"storj.io/storj/shared/location"
)

func TestEmptyIP2CountryMock(t *testing.T) {
	ipLookup := geoip.MockIPToCountry{}
	{
		co, err := ipLookup.LookupISOCountryCode("127.0.0.1")
		require.NoError(t, err)
		require.Equal(t, location.CountryCode(0), co)
	}

	{
		co, err := ipLookup.LookupISOCountryCode("[2001:0db8:85a3:0000:0000:8a2e:0370:7334]")
		require.NoError(t, err)
		require.Equal(t, location.CountryCode(0), co)
	}
}

func TestIP2CountryMock(t *testing.T) {
	ipLookup := geoip.MockIPToCountry{location.UnitedStates, location.Germany, location.France}

	cases := []struct {
		name        string
		address     string
		country     location.CountryCode
		errExpected bool
	}{
		{"first IP in the pool", "127.0.0.1:1234", location.Germany, false},
		{"second IP in the pool", "127.0.0.2:1234", location.France, false},
		{"third IP in the pool", "127.0.0.3:1234", location.UnitedStates, false},
		{"ipv6", "[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:1234", location.Germany, false},
		{"not an ip address", "not at all", location.CountryCode(0), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			co, err := ipLookup.LookupISOCountryCode(tc.address)
			if tc.errExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.country, co)
			}
		})

	}
}
