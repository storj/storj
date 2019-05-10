package console_test

import (
	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"testing"
)

func TestNewRegistrationSecret(t *testing.T) {
	// testing constants
	const (
		// for user
		shortName    = "lastName"
		email        = "email@ukr.net"
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
				FullName:     userFullName,
				ShortName:    shortName,
				Email:        email,
				PasswordHash: []byte(pass),
			})
			assert.NoError(t, err)
			assert.NotNil(t, owner)

			rptoken, err = rptokens.Create(ctx, &owner.ID)
			assert.NotNil(t, rptoken)
			assert.NoError(t, err)
		})

		t.Run("Get reset password token successfully", func(t *testing.T) {
			tokenBySecret, err := rptokens.GetBySecret(ctx, rptoken.Secret)
			assert.NoError(t, err)
			assert.Equal(t, rptoken.Secret, tokenBySecret.Secret)
			assert.Equal(t, rptoken.CreatedAt, tokenBySecret.CreatedAt)
			assert.Equal(t, rptoken.OwnerId, tokenBySecret.OwnerId)
		})

		t.Run("Get reset password token by UUID and Secret equal", func(t *testing.T) {
			tokenBySecret, err := rptokens.GetBySecret(ctx, rptoken.Secret)
			assert.NoError(t, err)

			tokenByUUID, err := rptokens.GetByOwnerID(ctx, *rptoken.OwnerId)
			assert.NoError(t, err)

			assert.Equal(t, tokenByUUID.Secret, tokenBySecret.Secret)
			assert.Equal(t, tokenByUUID.CreatedAt, tokenBySecret.CreatedAt)
			assert.Equal(t, tokenByUUID.OwnerId, tokenBySecret.OwnerId)
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
