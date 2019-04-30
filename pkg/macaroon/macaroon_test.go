// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package macaroon_test

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"storj.io/storj/pkg/macaroon"
)

func TestNilMacaroon(t *testing.T) {
	mac, err := macaroon.NewUnrestricted(nil)
	assert.NoError(t, err)
	assert.NotNil(t, mac)
	data := mac.Serialize()
	assert.NotNil(t, data)
	assert.NotEmpty(t, data)
	mac2, err := macaroon.ParseMacaroon(data)
	assert.NoError(t, err)
	assert.NotNil(t, mac2)
	assert.Equal(t, mac, mac2)

	t.Run("Successful add Caveat", func(t *testing.T) {
		mac, err = mac.AddFirstPartyCaveat([]byte("cav1"))
		assert.NotNil(t, mac)
		assert.NoError(t, err)
		assert.Equal(t, len(mac.Caveats()), 1)
	})

	t.Run("Successful serialization", func(t *testing.T) {
		data := mac.Serialize()
		assert.NotNil(t, data)
		assert.NotEmpty(t, data)

		mac2, err := macaroon.ParseMacaroon(data)
		assert.NotNil(t, mac2)
		assert.NoError(t, err)
		assert.Equal(t, mac, mac2)
	})
}

func TestMacaroon(t *testing.T) {
	secret, err := macaroon.NewSecret()
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, len(secret), 32)

	mac, err := macaroon.NewUnrestricted(secret)
	assert.NoError(t, err)
	assert.NotNil(t, mac)

	nonce := mac.Head()
	assert.NotNil(t, nonce)
	assert.Equal(t, len(nonce), 32)

	t.Run("Successful add Caveat", func(t *testing.T) {
		mac, err = mac.AddFirstPartyCaveat([]byte("cav1"))
		assert.NotNil(t, mac)
		assert.NoError(t, err)
		assert.Equal(t, len(mac.Caveats()), 1)
	})

	t.Run("Successful serialization", func(t *testing.T) {
		data := mac.Serialize()
		assert.NotNil(t, data)
		assert.NotEmpty(t, data)

		mac2, err := macaroon.ParseMacaroon(data)
		assert.NotNil(t, mac2)
		assert.NoError(t, err)
		assert.Equal(t, mac, mac2)
	})

	t.Run("Successful Unpack", func(t *testing.T) {
		ok := mac.Validate(secret)
		assert.True(t, ok)
		c := mac.Caveats()
		assert.NotNil(t, c)
		assert.NotEmpty(t, c)
	})
}
