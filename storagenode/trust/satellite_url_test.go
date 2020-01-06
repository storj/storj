// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/trust"
)

func TestSatelliteURLAddress(t *testing.T) {
	satelliteURL, err := trust.ParseSatelliteURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777")
	require.NoError(t, err)

	require.Equal(t, "127.0.0.1:7777", satelliteURL.Address())
}

func TestSatelliteURLString(t *testing.T) {
	satelliteURL, err := trust.ParseSatelliteURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777")
	require.NoError(t, err)

	require.Equal(t, "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777", satelliteURL.String())
}

func TestSatelliteURLNodeURLConversion(t *testing.T) {
	nodeURL, err := storj.ParseNodeURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777")
	require.NoError(t, err)

	satelliteURL, err := trust.ParseSatelliteURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777")
	require.NoError(t, err)

	require.Equal(t, nodeURL, satelliteURL.NodeURL())
}

func TestParseSatelliteURL(t *testing.T) {
	id, err := storj.NodeIDFromString("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6")
	require.NoError(t, err)

	for _, tt := range []struct {
		name   string
		url    string
		parsed trust.SatelliteURL
		err    string
	}{
		{
			name: "not a valid URL",
			url:  "://",
			err:  `invalid satellite URL: node URL error: parse ://: missing protocol scheme`,
		},
		{
			name: "missing ID",
			url:  "127.0.0.1:7777",
			err:  "invalid satellite URL: must contain an ID",
		},
		{
			name: "short ID",
			url:  "121RTSDpy@127.0.0.1:7777",
			err:  "invalid satellite URL: node URL error: node ID error: checksum error",
		},
		{
			name: "missing host:port",
			url:  "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@",
			err:  "invalid satellite URL: must specify the host:port",
		},
		{
			name: "missing port",
			url:  "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1",
			err:  "invalid satellite URL: must specify the port",
		},
		{
			name: "success",
			url:  "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777",
			parsed: trust.SatelliteURL{
				ID:   id,
				Host: "127.0.0.1",
				Port: 7777,
			},
		},
		{
			name: "success with storj schema",
			url:  "storj://121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777/",
			parsed: trust.SatelliteURL{
				ID:   id,
				Host: "127.0.0.1",
				Port: 7777,
			},
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			u, err := trust.ParseSatelliteURL(tt.url)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.parsed, u)
		})
	}

}
