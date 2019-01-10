// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToken(t *testing.T) {
	token := Token{
		Payload:   []byte{1, 2, 3},
		Signature: []byte{4, 5, 6},
	}

	tokenString := token.String()
	assert.NotNil(t, tokenString)
	assert.Equal(t, len(tokenString) > 0, true)

	tokenFromString, err := FromBase64URLString(tokenString)
	assert.Nil(t, err)
	assert.NoError(t, err)
	assert.Equal(t, tokenFromString.Payload, token.Payload)
	assert.Equal(t, tokenFromString.Signature, token.Signature)
}
