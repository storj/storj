// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUplink(t *testing.T) {
	var cerr Cpchar

	var config CUplinkConfig
	config.Volatile.TLS.SkipPeerCAWhitelist = Cbool(true)

	uplink := new_uplink(config, &cerr)
	require.Nil(t, cerr)
	require.NotEmpty(t, uplink)

	close_uplink(uplink, &cerr)
	require.Nil(t, cerr)
}