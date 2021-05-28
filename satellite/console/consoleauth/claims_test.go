// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleauth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testrand"
)

func TestClaims(t *testing.T) {
	id := testrand.UUID()

	claims := Claims{
		ID:         id,
		Email:      "alice@mail.test",
		Expiration: time.Now(),
	}

	claimsBytes, err := claims.JSON()
	assert.NoError(t, err)
	assert.NotNil(t, claimsBytes)

	parsedClaims, err := FromJSON(claimsBytes)
	assert.NoError(t, err)
	assert.NotNil(t, parsedClaims)

	assert.Equal(t, parsedClaims.Email, claims.Email)
	assert.Equal(t, parsedClaims.ID, claims.ID)
	assert.Equal(t, parsedClaims.Expiration.Year(), claims.Expiration.Year())
	assert.Equal(t, parsedClaims.Expiration.Month(), claims.Expiration.Month())
	assert.Equal(t, parsedClaims.Expiration.Day(), claims.Expiration.Day())
	assert.Equal(t, parsedClaims.Expiration.Hour(), claims.Expiration.Hour())
	assert.Equal(t, parsedClaims.Expiration.Minute(), claims.Expiration.Minute())
	assert.Equal(t, parsedClaims.Expiration.Second(), claims.Expiration.Second())
}
