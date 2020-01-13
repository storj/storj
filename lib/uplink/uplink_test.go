// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testcontext"
)

// TestUplinkConfigDefaults tests that the uplink configuration gets the correct defaults applied
// and that the defaults get applied all the way down to the transport layer.
func TestUplinkConfigDefaultTimeouts(t *testing.T) {
	ctx := testcontext.New(t)
	cfg := &Config{}
	client, err := NewUplink(ctx, cfg)

	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Assert the lib uplink configuration gets the correct defaults applied.
	assert.Equal(t, 20*time.Second, client.cfg.Volatile.DialTimeout)

	// Assert the values propagate correctly all the way down to the transport layer.
	assert.Equal(t, 20*time.Second, client.dialer.DialTimeout)
}

// TestUplinkConfigSetTimeouts tests that the uplink configuration settings properly override
// the defaults all the way down to the transport layer.
func TestUplinkConfigSetTimeouts(t *testing.T) {
	ctx := testcontext.New(t)

	cfg := &Config{}
	cfg.Volatile.DialTimeout = 120 * time.Second
	cfg.Volatile.TLS = struct {
		SkipPeerCAWhitelist bool
		PeerCAWhitelistPath string
	}{
		SkipPeerCAWhitelist: false,
		PeerCAWhitelistPath: "",
	}

	client, err := NewUplink(ctx, cfg)

	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Assert the lib uplink configuration gets the correct values applied.
	assert.Equal(t, 120*time.Second, client.cfg.Volatile.DialTimeout)

	// Assert the values propagate correctly all the way down to the transport layer.
	assert.Equal(t, 120*time.Second, client.dialer.DialTimeout)
}
