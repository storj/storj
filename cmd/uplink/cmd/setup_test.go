// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package cmd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/cmd/uplink/cmd"
)

func TestApplyDefaultHostAndPortToAddr(t *testing.T) {
	{
		got, err := cmd.ApplyDefaultHostAndPortToAddr("", "localhost:7777")
		assert.NoError(t, err)
		assert.Equal(t, "localhost:7777", got,
			"satellite-addr should contain default port when no port specified")
	}

	{
		got, err := cmd.ApplyDefaultHostAndPortToAddr("ahost", "localhost:7777")
		assert.NoError(t, err)
		assert.Equal(t, "ahost:7777", got,
			"satellite-addr should contain default port when no port specified")
	}

	{
		got, err := cmd.ApplyDefaultHostAndPortToAddr("ahost:7778", "localhost:7777")
		assert.NoError(t, err)
		assert.Equal(t, "ahost:7778", got,
			"satellite-addr should contain default port when no port specified")
	}

	{
		got, err := cmd.ApplyDefaultHostAndPortToAddr(":7778", "localhost:7777")
		assert.NoError(t, err)
		assert.Equal(t, "localhost:7778", got,
			"satellite-addr should contain default port when no port specified")
	}

	{
		got, err := cmd.ApplyDefaultHostAndPortToAddr("ahost:", "localhost:7777")
		assert.NoError(t, err)
		assert.Equal(t, "ahost:7777", got,
			"satellite-addr should contain default port when no port specified")
	}
}
