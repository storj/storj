// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package revocation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/macaroon"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestRevocation(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		secret, err := macaroon.NewSecret()
		require.NoError(t, err)

		// mac: original macaroon
		mac, err := macaroon.NewUnrestricted(secret)
		require.NoError(t, err)

		// mac1 based on mac
		mac1, err := mac.AddFirstPartyCaveat([]byte("this is a very serious caveat, you'd better not violate it"))
		require.NoError(t, err)

		// mac2 based on mac
		mac2, err := mac.AddFirstPartyCaveat([]byte("don't mess with this caveat"))
		require.NoError(t, err)

		// mac1a based on mac1
		mac1a, err := mac1.AddFirstPartyCaveat([]byte("now you can't do anything"))
		require.NoError(t, err)

		revocation := db.Revocation()

		// Check all macaroons as sanity check, they work before revocation
		for _, mac := range []*macaroon.Macaroon{mac, mac1, mac2, mac1a} {
			revoked, err := revocation.Check(ctx, mac.Tails(secret))
			require.NoError(t, err)
			assert.False(t, revoked)
		}

		apiKeyID := []byte("api1")

		// Now revoke mac1, which should also revoke mac1a but not affect mac or mac2
		require.NoError(t, revocation.Revoke(ctx, mac1.Tail(), apiKeyID))
		// Also revoke some random bytes, so the db has more than 1 entry
		require.NoError(t, revocation.Revoke(ctx, []byte("random tail"), apiKeyID))
		require.NoError(t, revocation.Revoke(ctx, []byte("random tail2"), apiKeyID))

		// Verify mac1 and mac1a got revoked
		for _, mac := range []*macaroon.Macaroon{mac1, mac1a} {
			revoked, err := revocation.Check(ctx, mac.Tails(secret))
			require.NoError(t, err)
			assert.True(t, revoked)
		}

		// Verify mac and mac2 are not revoked
		for _, mac := range []*macaroon.Macaroon{mac, mac2} {
			revoked, err := revocation.Check(ctx, mac.Tails(secret))
			require.NoError(t, err)
			assert.False(t, revoked)
		}
	})
}
