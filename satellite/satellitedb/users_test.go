// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

func TestUserFromDbx(t *testing.T) {
	t.Run("can't create dbo from nil dbx model", func(t *testing.T) {
		user, err := userFromDBX(nil)
		assert.Nil(t, user)
		assert.Error(t, err)
	})

	t.Run("can't create dbo from dbx model with invalid ID", func(t *testing.T) {
		dbxUser := dbx.User{
			Id:           []byte("qweqwe"),
			FullName:     "Very long full name",
			ShortName:    nil,
			Email:        "some@email.com",
			PasswordHash: []byte("ihqerfgnu238723huagsd"),
			CreatedAt:    time.Now(),
		}

		user, err := userFromDBX(&dbxUser)

		assert.Nil(t, user)
		assert.Error(t, err)
	})
}
