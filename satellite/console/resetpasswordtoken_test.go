// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestNewRegistrationSecret(t *testing.T) {
	// testing constants
	const (
		// for user
		shortName    = "lastName"
		email        = "email@mail.test"
		pass         = "123456"
		userFullName = "name"
	)

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		users := db.Console().Users()
		rptokens := db.Console().ResetPasswordTokens()

		var owner *console.User
		var rptoken *console.ResetPasswordToken

		t.Run("Insert reset password token successfully", func(t *testing.T) {
			var err error
			owner, err = users.Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				FullName:     userFullName,
				ShortName:    shortName,
				Email:        email,
				PasswordHash: []byte(pass),
			})
			assert.NoError(t, err)
			assert.NotNil(t, owner)

			rptoken, err = rptokens.Create(ctx, owner.ID)
			assert.NotNil(t, rptoken)
			assert.NoError(t, err)
		})

		t.Run("Get reset password token successfully", func(t *testing.T) {
			tokenBySecret, err := rptokens.GetBySecret(ctx, rptoken.Secret)
			assert.NoError(t, err)
			assert.Equal(t, rptoken.Secret, tokenBySecret.Secret)
			assert.Equal(t, rptoken.CreatedAt, tokenBySecret.CreatedAt)
			assert.Equal(t, rptoken.OwnerID, tokenBySecret.OwnerID)
		})

		t.Run("Get reset password token by UUID and Secret equal", func(t *testing.T) {
			tokenBySecret, err := rptokens.GetBySecret(ctx, rptoken.Secret)
			assert.NoError(t, err)

			tokenByUUID, err := rptokens.GetByOwnerID(ctx, *rptoken.OwnerID)
			assert.NoError(t, err)

			assert.Equal(t, tokenByUUID.Secret, tokenBySecret.Secret)
			assert.Equal(t, tokenByUUID.CreatedAt, tokenBySecret.CreatedAt)
			assert.Equal(t, tokenByUUID.OwnerID, tokenBySecret.OwnerID)
		})

		t.Run("Successful base64 encoding", func(t *testing.T) {
			base64token := rptoken.Secret.String()
			assert.NotEmpty(t, base64token)
		})

		t.Run("Successful base64 decoding", func(t *testing.T) {
			base64token := rptoken.Secret.String()
			assert.NotEmpty(t, base64token)

			secretFromString, err := console.ResetPasswordSecretFromBase64(base64token)

			assert.NoError(t, err)
			assert.Equal(t, rptoken.Secret, secretFromString)
		})

	})
}
