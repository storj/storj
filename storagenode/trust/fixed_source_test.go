// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixedSource(t *testing.T) {
	satelliteURL, err := ParseSatelliteURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@us-central-1.tardigrade.io:7777")
	require.NoError(t, err)

	for _, tt := range []struct {
		name    string
		url     string
		err     string
		entries []Entry
	}{
		{
			name: "incomplete satellite URL",
			url:  "us-central-1.tardigrade.io:7777",
			err:  "trust: invalid satellite URL: must contain an ID",
		},
		{
			name: "good satellite URL",
			url:  "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@us-central-1.tardigrade.io:7777",
			entries: []Entry{
				{
					SatelliteURL:  satelliteURL,
					Authoritative: true,
				},
			},
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			source, err := NewFixedSource(tt.url)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)

			require.True(t, source.Fixed(), "fixed source is not fixed")
			require.Equal(t, tt.url, source.String(), "fixed source string should match url")

			entries, err := source.FetchEntries(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.entries, entries)
		})
	}
}
