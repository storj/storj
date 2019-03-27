// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testpeertls

import (
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/pkcrypto"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/storj"
)

func TestNewCertChain(t *testing.T) {
	testidentity.CompleteIdentityVersionsTest(t, func(t *testing.T, _ storj.IDVersion, ident *identity.FullIdentity) {
		length := 2
		//for length := 2; length < 4; length ++ {
		//	t.Logf("length: %d", length)
		keys, chain, err := NewCertChain(length, storj.V2)
		require.NoError(t, err)

		assert.Len(t, chain, length)
		assert.Len(t, keys, length)

		assert.Equal(t, pkcrypto.PublicKeyFromPrivate(keys[peertls.CAIndex]), chain[peertls.CAIndex].PublicKey)
		assert.Equal(t, pkcrypto.PublicKeyFromPrivate(keys[peertls.LeafIndex]), chain[peertls.LeafIndex].PublicKey)

		err = peertls.VerifyPeerCertChains(nil, identity.ToChains(chain))
		assert.NoError(t, err)

		assert.True(t, chain[peertls.CAIndex].IsCA)
		assert.False(t, chain[peertls.LeafIndex].IsCA)
		//}
	})
}
