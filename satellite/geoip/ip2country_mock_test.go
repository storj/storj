// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package geoip_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/geoip"
)

func TestIP2CountryMock(t *testing.T) {
	ipLookup := geoip.MockIPToCountry{"US", "DE", "FR"}

	{
		co, err := ipLookup.LookupISOCountryCode("127.0.0.1")
		require.NoError(t, err)
		require.Equal(t, "DE", co)
	}

	{
		co, err := ipLookup.LookupISOCountryCode("127.0.0.2:1234")
		require.NoError(t, err)
		require.Equal(t, "FR", co)
	}

	{
		co, err := ipLookup.LookupISOCountryCode("127.0.0.3")
		require.NoError(t, err)
		require.Equal(t, "US", co)
	}

	{
		co, err := ipLookup.LookupISOCountryCode("[2001:0db8:85a3:0000:0000:8a2e:0370:7334]")
		require.NoError(t, err)
		require.Equal(t, "DE", co)
	}

	{
		co, err := ipLookup.LookupISOCountryCode("[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:1234")
		require.NoError(t, err)
		require.Equal(t, "DE", co)
	}

	{
		co, err := ipLookup.LookupISOCountryCode("not an ipaddress")
		require.NoError(t, err)
		require.Equal(t, "", co)
	}
}