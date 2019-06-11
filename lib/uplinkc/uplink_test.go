// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewUplink(t *testing.T) {
	var cerr CPChar

	var config CUplinkConfig
	config.Volatile.TLS.SkipPeerCAWhitelist = 1

	uplink := NewUplink(config, &cerr)
	require.Nil(t, cerr)
	require.NotEmpty(t, uplink)

	CloseUplink(uplink, &cerr)
	require.Nil(t, cerr)
}
