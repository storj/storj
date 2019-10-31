// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testrand"
)

func TestListConfig(t *testing.T) {
	var config ListConfig
	assert.Equal(t, "trust-list", config.Type())
	assert.Equal(t, "", config.String())

	// Assert that comma separated values can be set
	require.NoError(t, config.Set("12ugxzSYW7UA1n7jfVyJHV2zL5TL1x5fSYZb48y7jZ1r3bLhbfo@127.0.0.1:12345,!foo.test"))
	assert.Equal(t, "12ugxzSYW7UA1n7jfVyJHV2zL5TL1x5fSYZb48y7jZ1r3bLhbfo@127.0.0.1:12345,!foo.test", config.String())

	// Assert that a failure to set does not modify the current values
	require.Error(t, config.Set("-"))
	assert.Equal(t, "12ugxzSYW7UA1n7jfVyJHV2zL5TL1x5fSYZb48y7jZ1r3bLhbfo@127.0.0.1:12345,!foo.test", config.String())

	// Assert the source was configured correctly
	if assert.Len(t, config.Sources, 1) {
		assert.Equal(t, "12ugxzSYW7UA1n7jfVyJHV2zL5TL1x5fSYZb48y7jZ1r3bLhbfo@127.0.0.1:12345", config.Sources[0].String())
	}

	// Assert the filter was configured correctly (i.e. foo.test does not pass the filter)
	assert.False(t, config.Filter.Passes(SatelliteURL{
		Host: "foo.test",
	}))
}

func TestParseConfigListInvalidFilter(t *testing.T) {
	_, _, err := ParseConfigList([]string{"!"})
	require.EqualError(t, err, "trust: invalid filter at position 0: satellite URL filter must specify a port")
}

func TestParseConfigListInvalidSource(t *testing.T) {
	_, _, err := ParseConfigList([]string{""})
	require.EqualError(t, err, "trust: invalid source at position 0: invalid satellite URL: must contain an ID")
}

func TestParseConfigList(t *testing.T) {
	a := testrand.NodeID()
	b := testrand.NodeID()
	c := testrand.NodeID()
	d := testrand.NodeID()

	list := []string{
		"!quz.test",
		"file:///path/to/some/trusted-satellites.txt",
		"https://foo.test/trusted-satellites",
		"https://bar.test/trusted-satellites",
		"https://baz.test/trusted-satellites",
		fmt.Sprintf("%s@f.foo.test:7777", a.String()),
		fmt.Sprintf("!%s@qiz.test:7777", b.String()),
		fmt.Sprintf("!%s@", c.String()),
	}

	sources, filter, err := ParseConfigList(list)
	require.NoError(t, err)

	// assert sources are returned in order specified
	require.Len(t, sources, 5)
	require.Equal(t, "file:///path/to/some/trusted-satellites.txt", sources[0].String())
	require.Equal(t, "https://foo.test/trusted-satellites", sources[1].String())
	require.Equal(t, "https://bar.test/trusted-satellites", sources[2].String())
	require.Equal(t, "https://baz.test/trusted-satellites", sources[3].String())
	require.Equal(t, fmt.Sprintf("%s@f.foo.test:7777", a), sources[4].String())

	// assert filter was set up properly
	require.False(t, filter.Passes(SatelliteURL{ID: d, Host: "quz.test", Port: 7777}))
	require.False(t, filter.Passes(SatelliteURL{ID: b, Host: "qiz.test", Port: 7777}))
	require.False(t, filter.Passes(SatelliteURL{ID: c, Host: "whatever.test", Port: 7777}))
}
