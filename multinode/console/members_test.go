// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/multinode"
	"storj.io/storj/multinode/console"
	"storj.io/storj/multinode/multinodedb/multinodedbtest"
)

func TestMembersDB(t *testing.T) {
	multinodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db multinode.DB) {
		members := db.Members()

		memberID, err := uuid.New()
		require.NoError(t, err)

		memberBob := console.Member{
			ID:           memberID,
			Email:        "mail@example.com",
			Name:         "Bob",
			PasswordHash: []byte{0},
		}

		err = members.Invite(ctx, memberBob)
		assert.NoError(t, err)

		memberToCheck, err := members.GetByEmail(ctx, memberBob.Email)
		assert.NoError(t, err)
		assert.Equal(t, memberToCheck.Email, memberBob.Email)
		assert.Equal(t, memberToCheck.Name, memberBob.Name)
		assert.Equal(t, memberToCheck.Email, memberBob.Email)

		memberBob.Name = "Alice"
		err = members.Update(ctx, memberBob)
		assert.NoError(t, err)

		memberAlice, err := members.GetByID(ctx, memberToCheck.ID)
		assert.NoError(t, err)
		assert.Equal(t, memberToCheck.Email, memberAlice.Email)
		assert.Equal(t, memberToCheck.Name, memberAlice.Name)
		assert.Equal(t, memberToCheck.Email, memberAlice.Email)
		assert.Equal(t, memberToCheck.ID, memberAlice.ID)

		err = members.Remove(ctx, memberAlice.ID)
		assert.NoError(t, err)

		_, err = members.GetByID(ctx, memberToCheck.ID)
		assert.Error(t, err)
		assert.Equal(t, true, console.ErrNoMember.Has(err))

		_, err = members.GetByEmail(ctx, memberToCheck.Email)
		assert.Error(t, err)
		assert.Equal(t, true, console.ErrNoMember.Has(err))
	})
}
