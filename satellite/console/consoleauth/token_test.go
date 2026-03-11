// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleauth

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/uuid"
)

func TestParseSessionPayload(t *testing.T) {
	t.Run("OldFormat", func(t *testing.T) {
		id, err := uuid.New()
		require.NoError(t, err)

		p, err := ParseSessionPayload(id.Bytes())
		require.NoError(t, err)
		assert.Equal(t, id, p.SessionID)
		assert.Empty(t, p.IDPToken)
		assert.True(t, p.IDPTokenExpiry.IsZero())
		assert.Empty(t, p.IDPRefreshToken)
	})

	t.Run("NewFormat", func(t *testing.T) {
		id, err := uuid.New()
		require.NoError(t, err)

		expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
		payload, err := json.Marshal(SessionPayload{
			SessionID:       id,
			IDPToken:        "test-idp-access-token",
			IDPTokenExpiry:  expiry,
			IDPRefreshToken: "test-idp-refresh-token",
		})
		require.NoError(t, err)

		p, err := ParseSessionPayload(payload)
		require.NoError(t, err)
		assert.Equal(t, id, p.SessionID)
		assert.Equal(t, "test-idp-access-token", p.IDPToken)
		assert.Equal(t, expiry, p.IDPTokenExpiry.UTC().Truncate(time.Second))
		assert.Equal(t, "test-idp-refresh-token", p.IDPRefreshToken)
	})

	t.Run("InvalidInput", func(t *testing.T) {
		_, err := ParseSessionPayload([]byte("not-valid"))
		require.Error(t, err)
	})
}

func TestToken(t *testing.T) {
	token := Token{
		Payload:   []byte{1, 2, 3},
		Signature: []byte{4, 5, 6},
	}

	tokenString := token.String()
	assert.NotNil(t, tokenString)
	assert.Equal(t, len(tokenString) > 0, true)

	tokenFromString, err := FromBase64URLString(tokenString)
	assert.NoError(t, err)
	assert.Equal(t, tokenFromString.Payload, token.Payload)
	assert.Equal(t, tokenFromString.Signature, token.Signature)
}
