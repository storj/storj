// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testrand"
)

func TestNotificationID(t *testing.T) {
	satellite0 := testrand.NodeID()
	satellite1 := testrand.NodeID()

	t.Run("stable for the same sender and key", func(t *testing.T) {
		require.Equal(t,
			notificationID(satellite0, "maintenance"),
			notificationID(satellite0, "maintenance"))
	})

	t.Run("differs per satellite", func(t *testing.T) {
		require.NotEqual(t,
			notificationID(satellite0, "maintenance"),
			notificationID(satellite1, "maintenance"))
	})

	t.Run("differs per key", func(t *testing.T) {
		require.NotEqual(t,
			notificationID(satellite0, "maintenance"),
			notificationID(satellite0, "policy-change"))
	})

	t.Run("is a well formed v5 UUID", func(t *testing.T) {
		id := notificationID(satellite0, "maintenance")
		require.Equal(t, byte(0x50), id[6]&0xf0, "version nibble must be 5")
		require.Equal(t, byte(0x80), id[8]&0xc0, "variant bits must be RFC 4122")
		require.False(t, id.IsZero())
	})

	// The sender is a fixed width prefix, so no key can shift into it and
	// collide with a different sender/key pair.
	t.Run("no ambiguity between sender and key", func(t *testing.T) {
		var zero storj.NodeID
		require.NotEqual(t,
			notificationID(zero, "ab"),
			notificationID(zero, "a")) // would collide under naive concatenation
	})
}
