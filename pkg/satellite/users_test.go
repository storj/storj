// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/satellite/satellitedb/dbx"
)

func TestUserDboFromDbx(t *testing.T) {

	t.Run("can't create dbo from nil dbx model", func(t *testing.T) {
		user, err := UserFromDBX(nil)

		assert.Nil(t, user)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("can't create dbo from dbx model with invalid Id", func(t *testing.T) {
		dbxUser := dbx.User{
			Id:           "qweqwe",
			FirstName:    "FirstName",
			LastName:     "LastName",
			Email:        "email@ukr.net",
			PasswordHash: []byte("ihqerfgnu238723huagsd"),
			CreatedAt:    time.Now(),
		}

		user, err := UserFromDBX(&dbxUser)

		assert.Nil(t, user)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})
}
