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

		sessionID, idpToken, idpTokenExpiry, err := ParseSessionPayload(id.Bytes())
		require.NoError(t, err)
		assert.Equal(t, id, sessionID)
		assert.Empty(t, idpToken)
		assert.True(t, idpTokenExpiry.IsZero())
	})

	t.Run("NewJSONFormat", func(t *testing.T) {
		id, err := uuid.New()
		require.NoError(t, err)

		payload, err := json.Marshal(SessionPayload{
			SessionID: id.String(),
			IDPToken:  "test-idp-access-token",
		})
		require.NoError(t, err)

		sessionID, idpToken, idpTokenExpiry, err := ParseSessionPayload(payload)
		require.NoError(t, err)
		assert.Equal(t, id, sessionID)
		assert.Equal(t, "test-idp-access-token", idpToken)
		assert.True(t, idpTokenExpiry.IsZero())
	})

	t.Run("NewJSONFormatWithExpiry", func(t *testing.T) {
		id, err := uuid.New()
		require.NoError(t, err)

		expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
		payload, err := json.Marshal(SessionPayload{
			SessionID:      id.String(),
			IDPToken:       "test-idp-access-token",
			IDPTokenExpiry: expiry,
		})
		require.NoError(t, err)

		sessionID, idpToken, idpTokenExpiry, err := ParseSessionPayload(payload)
		require.NoError(t, err)
		assert.Equal(t, id, sessionID)
		assert.Equal(t, "test-idp-access-token", idpToken)
		assert.Equal(t, expiry, idpTokenExpiry.UTC().Truncate(time.Second))
	})

	t.Run("InvalidInput", func(t *testing.T) {
		_, _, _, err := ParseSessionPayload([]byte("not-valid"))
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
