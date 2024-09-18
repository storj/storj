// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information

package geoip

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/location"
)

func TestToCountryCode(t *testing.T) {
	require.Equal(t, location.Germany, toCountryCode(&ipInfo{
		Country: country{
			IsoCode: "DE",
		},
	}))

	require.Equal(t, location.Germany, toCountryCode(&ipInfo{
		Country: country{
			IsoCode: "DE",
		},
		RepresentedCountry: country{
			IsoCode: "DE",
		},
	}))

	require.Equal(t, location.None, toCountryCode(&ipInfo{
		Country: country{
			IsoCode: "DE",
		},
		RepresentedCountry: country{
			IsoCode: "US",
		},
	}))
}

func TestMaxmind(t *testing.T) {
	maxmindDB := os.Getenv("TEST_MAXMIND_DB")
	if maxmindDB == "" {
		t.Skip("Optional test")
	}

	db, err := OpenMaxmindDB(maxmindDB)
	require.NoError(t, err)

	// these assertions are based on the db from 2023-08-04. Can be different with different DB.

	code, err := db.LookupISOCountryCode("62.112.192.4:80")
	require.NoError(t, err)
	require.Equal(t, location.Hungary, code)

	code, err = db.LookupISOCountryCode("178.76.189.106:28967")
	require.NoError(t, err)
	require.Equal(t, location.None, code)
}
