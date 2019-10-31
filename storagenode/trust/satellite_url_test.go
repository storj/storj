// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/storj"
)

func TestSatelliteURLAddress(t *testing.T) {
	satelliteURL, err := ParseSatelliteURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777")
	require.NoError(t, err)

	require.Equal(t, "127.0.0.1:7777", satelliteURL.Address())
}

func TestSatelliteURLNodeURLConversion(t *testing.T) {
	nodeURL, err := storj.ParseNodeURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777")
	require.NoError(t, err)

	satelliteURL, err := ParseSatelliteURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777")
	require.NoError(t, err)

	require.Equal(t, nodeURL, satelliteURL.NodeURL())
}

func TestParseSatelliteURL(t *testing.T) {
	id, err := storj.NodeIDFromString("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6")
	require.NoError(t, err)

	for _, tt := range []struct {
		name   string
		url    string
		parsed SatelliteURL
		err    string
	}{
		{
			name: "not a valid URL",
			url:  "://",
			err:  `trust: invalid satellite URL: node URL error: parse ://: missing protocol scheme`,
		},
		{
			name: "missing ID",
			url:  "127.0.0.1:7777",
			err:  "trust: invalid satellite URL: must contain an ID",
		},
		{
			name: "missing port",
			url:  "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1",
			err:  "trust: invalid satellite URL: must specify the port",
		},
		{
			name: "success",
			url:  "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777",
			parsed: SatelliteURL{
				ID:   id,
				Host: "127.0.0.1",
				Port: 7777,
			},
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			u, err := ParseSatelliteURL(tt.url)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.parsed, u)
		})
	}

}
