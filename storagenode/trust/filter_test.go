// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterAddFailure(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config string
		err    string
	}{
		{
			name:   "not a valid URL",
			config: "://",
			err:    "trust: invalid filter: node URL error: parse ://: missing protocol scheme",
		},
		{
			name:   "host filter must not specify a port",
			config: "bar.test:7777",
			err:    "trust: host filter must not specify a port",
		},
		{
			name:   "satellite URL filter must specify a port",
			config: "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@bar.test",
			err:    "trust: satellite URL filter must specify a port",
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			filter := NewFilter()

			err := filter.Add(tt.config)
			require.EqualError(t, err, tt.err)
		})
	}

}

func TestFilterPasses(t *testing.T) {
	goodURL, err := ParseSatelliteURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@foo.test:7777")
	require.NoError(t, err)

	badURL, err := ParseSatelliteURL("12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@b.bar.test:7777")
	require.NoError(t, err)

	for _, tt := range []struct {
		name   string
		config string
	}{
		{
			name:   "filtered by id",
			config: "12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@",
		},
		{
			name:   "filtered by root domain",
			config: "bar.test",
		},
		{
			name:   "filtered by exact domain",
			config: "b.bar.test",
		},
		{
			name:   "filtered by full url",
			config: "12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@b.bar.test:7777",
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			filter := NewFilter()

			err := filter.Add(tt.config)
			require.NoError(t, err)

			assert.True(t, filter.Passes(goodURL), "good URL should pass")
			assert.False(t, filter.Passes(badURL), "bad URL should not pass")
		})
	}
}
