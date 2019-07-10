// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
)

func TestPublicPrivatePieceKey(t *testing.T) {
	expectedPublicKey, expectedPrivateKey, err := storj.NewPieceKey()
	require.NoError(t, err)

	publicKey, err := storj.PiecePublicKeyFromBytes(expectedPublicKey.Bytes())
	require.NoError(t, err)
	require.Equal(t, expectedPublicKey, publicKey)

	privateKey, err := storj.PiecePrivateKeyFromBytes(expectedPrivateKey.Bytes())
	require.NoError(t, err)
	require.Equal(t, expectedPrivateKey, privateKey)

	{
		data := []byte("data to sign")
		signature := privateKey.Sign(data)

		verified := publicKey.Verify(data, signature)
		require.True(t, verified)

		verified = publicKey.Verify(data, []byte("invalid signature"))
		require.False(t, verified)

		verified = publicKey.Verify([]byte("invalid data"), signature)
		require.False(t, verified)
	}

	{
		// to small
		_, err = storj.PiecePublicKeyFromBytes([]byte{1})
		require.Error(t, err)

		// to small
		_, err = storj.PiecePrivateKeyFromBytes([]byte{1})
		require.Error(t, err)

		// to large
		_, err = storj.PiecePublicKeyFromBytes(testrand.Bytes(33))
		require.Error(t, err)

		// to large
		_, err = storj.PiecePrivateKeyFromBytes(testrand.Bytes(65))
		require.Error(t, err)

		// public key from private
		_, err = storj.PiecePublicKeyFromBytes(expectedPrivateKey.Bytes())
		require.Error(t, err)

		// private key from public
		_, err = storj.PiecePrivateKeyFromBytes(expectedPublicKey.Bytes())
		require.Error(t, err)
	}
}
