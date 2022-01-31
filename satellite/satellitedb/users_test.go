// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/satellite/satellitedb/dbx"
)

func TestUserFromDbx(t *testing.T) {
	ctx := context.Background()

	t.Run("can't create dbo from nil dbx model", func(t *testing.T) {
		user, err := userFromDBX(ctx, nil)
		assert.Nil(t, user)
		assert.Error(t, err)
	})

	t.Run("can't create dbo from dbx model with invalid ID", func(t *testing.T) {
		dbxUser := dbx.User{
			Id:           []byte("qweqwe"),
			FullName:     "Very long full name",
			ShortName:    nil,
			Email:        "some@mail.test",
			PasswordHash: []byte("ihqerfgnu238723huagsd"),
			CreatedAt:    time.Now(),
		}

		user, err := userFromDBX(ctx, &dbxUser)

		assert.Nil(t, user)
		assert.Error(t, err)
	})
}
