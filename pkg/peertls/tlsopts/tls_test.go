// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tlsopts_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/testidentity"
)

func TestVerifyIdentity_success(t *testing.T) {
	for i := 0; i < 50; i++ {
		ident, err := testidentity.PregeneratedIdentity(i, storj.LatestIDVersion())
		require.NoError(t, err)

		err = tlsopts.VerifyIdentity(ident.ID)(nil, identity.ToChains(ident.Chain()))
		assert.NoError(t, err)
	}
}

func TestVerifyIdentity_success_signed(t *testing.T) {
	for i := 0; i < 50; i++ {
		ident, err := testidentity.PregeneratedSignedIdentity(i, storj.LatestIDVersion())
		require.NoError(t, err)

		err = tlsopts.VerifyIdentity(ident.ID)(nil, identity.ToChains(ident.Chain()))
		assert.NoError(t, err)
	}
}

func TestVerifyIdentity_error(t *testing.T) {
	ident, err := testidentity.PregeneratedIdentity(0, storj.LatestIDVersion())
	require.NoError(t, err)

	identTheftVictim, err := testidentity.PregeneratedIdentity(1, storj.LatestIDVersion())
	require.NoError(t, err)

	cases := []struct {
		test   string
		nodeID storj.NodeID
	}{
		{"empty node ID", storj.NodeID{}},
		{"garbage node ID", storj.NodeID{0, 1, 2, 3}},
		{"wrong node ID", identTheftVictim.ID},
	}

	for _, cc := range cases {
		testCase := cc
		t.Run(testCase.test, func(t *testing.T) {
			err := tlsopts.VerifyIdentity(testCase.nodeID)(nil, identity.ToChains(ident.Chain()))
			assert.Error(t, err)
		})
	}
}
