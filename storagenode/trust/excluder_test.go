// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/storagenode/trust"
)

func TestNewExcluderFailure(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config string
		errs   []string
	}{
		{
			name:   "not a valid URL",
			config: "://",
			errs: []string{
				`exclusion: node URL error: parse ://: missing protocol scheme`,
				`exclusion: node URL error: parse "://": missing protocol scheme`,
				`exclusion: node URL: parse ://: missing protocol scheme`,
				`exclusion: node URL: parse "://": missing protocol scheme`,
			},
		},
		{
			name:   "host exclusion must not include a port",
			config: "bar.test:7777",
			errs:   []string{"exclusion: host exclusion must not include a port"},
		},
		{
			name:   "satellite URL exclusion must specify a port",
			config: "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@bar.test",
			errs:   []string{"exclusion: satellite URL exclusion must specify a port"},
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			_, err := trust.NewExcluder(tt.config)
			require.Error(t, err)
			require.Contains(t, tt.errs, err.Error())
		})
	}

}

func TestNewExcluder(t *testing.T) {
	goodURL := makeSatelliteURL("foo.test")
	badURL := makeSatelliteURL("b.bar.test")

	for _, tt := range []struct {
		name   string
		config string
	}{
		{
			name:   "filtered by id",
			config: badURL.ID.String() + "@",
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
			config: badURL.String(),
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			excluder, err := trust.NewExcluder(tt.config)
			require.NoError(t, err)

			assert.True(t, excluder.IsTrusted(goodURL), "good URL should not be excluded")
			assert.False(t, excluder.IsTrusted(badURL), "bad URL should be excluded")
		})
	}
}

func TestHostExcluder(t *testing.T) {
	for _, tt := range []struct {
		exclusion string
		host      string
		isTrusted bool
	}{
		{
			exclusion: "foo.test",
			host:      "foo.test",
			isTrusted: false,
		},
		{
			exclusion: "foo.test",
			host:      "x.foo.test",
			isTrusted: false,
		},
		{
			exclusion: "foo.test",
			host:      ".foo.test",
			isTrusted: false,
		},
		{
			exclusion: "foo.test",
			host:      "foo.test.",
			isTrusted: false,
		},
		{
			exclusion: "x.bar.test",
			host:      "bar.test",
			isTrusted: true,
		},
		{
			exclusion: "x.bar.test",
			host:      "x.bar.test",
			isTrusted: false,
		},
		{
			exclusion: "x.bar.test",
			host:      "y.x.bar.test",
			isTrusted: false,
		},
		{
			exclusion: ".baz.test",
			host:      "baz.test",
			isTrusted: false,
		},
		{
			exclusion: "baz.test.",
			host:      "baz.test",
			isTrusted: false,
		},
		{
			exclusion: "satellite",
			host:      "satellite",
			isTrusted: false,
		},
		{
			exclusion: "satellite",
			host:      "x.satellite",
			isTrusted: true,
		},
	} {
		tt := tt // quiet linting
		name := fmt.Sprintf("%s-%s-%t", tt.exclusion, tt.host, tt.isTrusted)
		t.Run(name, func(t *testing.T) {
			excluder := trust.NewHostExcluder(tt.exclusion)
			isTrusted := excluder.IsTrusted(trust.SatelliteURL{Host: tt.host})
			assert.Equal(t, tt.isTrusted, isTrusted)
		})
	}
}
