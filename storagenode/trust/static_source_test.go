// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/storagenode/trust"
)

func TestStaticURLSource(t *testing.T) {
	url := makeSatelliteURL("domain.test")

	for _, tt := range []struct {
		name    string
		url     string
		err     string
		entries []trust.Entry
	}{
		{
			name: "incomplete satellite URL",
			url:  "domain.test:7777",
			err:  "static source: invalid satellite URL: must contain an ID",
		},
		{
			name: "good satellite URL",
			url:  url.String(),
			entries: []trust.Entry{
				{
					SatelliteURL:  url,
					Authoritative: true,
				},
			},
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			source, err := trust.NewStaticURLSource(tt.url)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)

			require.True(t, source.Static(), "static source is not static")
			require.Equal(t, tt.url, source.String(), "static source string should match url")

			entries, err := source.FetchEntries(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tt.entries, entries)
		})
	}
}
