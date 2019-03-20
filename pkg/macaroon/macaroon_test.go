package macaroon_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/macaroon"
)

func TestNilMacaroon(t *testing.T) {
	mac := macaroon.NewUnrestricted(nil, nil)
	assert.NotNil(t, mac)
	data := macaroon.Serialize(mac)
	assert.NotNil(t, data)
	assert.NotEmpty(t, data)
	mac2, err := macaroon.Deserialize(data)
	assert.NoError(t, err)
	assert.NotNil(t, mac2)
	assert.Equal(t, mac, mac2)

	t.Run("Successful add Caveat", func(t *testing.T) {
		mac, err = mac.AddFirstPartyCaveat(macaroon.Caveat{Identifier: "cav1"})
		assert.NotNil(t, mac)
		assert.NoError(t, err)
		assert.Equal(t, len(mac.Caveats()), 1)
	})

	t.Run("Successful serialization", func(t *testing.T) {
		data := macaroon.Serialize(mac)
		assert.NotNil(t, data)
		assert.NotEmpty(t, data)

		mac2, err := macaroon.Deserialize(data)
		assert.NotNil(t, mac2)
		assert.NoError(t, err)
		assert.Equal(t, mac, mac2)
	})
}

func TestMacaroon(t *testing.T) {
	nonce, err := macaroon.NewNonce()
	assert.NotNil(t, nonce)
	assert.NoError(t, err)
	assert.Equal(t, len(nonce), 32)

	secret, err := macaroon.NewSecret()
	assert.NotNil(t, secret)
	assert.NoError(t, err)
	assert.Equal(t, len(secret), 32)

	mac := macaroon.NewUnrestricted(nonce, secret)
	assert.NotNil(t, mac)

	t.Run("Successful add Caveat", func(t *testing.T) {
		mac, err = mac.AddFirstPartyCaveat(macaroon.Caveat{Identifier: "cav1"})
		assert.NotNil(t, mac)
		assert.NoError(t, err)
		assert.Equal(t, len(mac.Caveats()), 1)
	})

	t.Run("Successful serialization", func(t *testing.T) {
		data := macaroon.Serialize(mac)
		assert.NotNil(t, data)
		assert.NotEmpty(t, data)

		mac2, err := macaroon.Deserialize(data)
		assert.NotNil(t, mac2)
		assert.NoError(t, err)
		assert.Equal(t, mac, mac2)
	})

	t.Run("Successful Unpack", func(t *testing.T) {
		c, ok := macaroon.CheckUnpack(secret, mac)
		assert.True(t, ok)
		assert.NotNil(t, c)
		assert.NotEmpty(t, c)
		assert.Equal(t, c, mac.Caveats())
	})
}
