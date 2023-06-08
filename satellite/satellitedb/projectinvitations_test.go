// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProjectInvitations(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		invitesDB := db.Console().ProjectInvitations()
		projectsDB := db.Console().Projects()

		inviterID := testrand.UUID()
		projID := testrand.UUID()
		projID2 := testrand.UUID()
		email := "user@mail.test"
		email2 := "user2@mail.test"

		_, err := projectsDB.Insert(ctx, &console.Project{ID: projID})
		require.NoError(t, err)
		_, err = projectsDB.Insert(ctx, &console.Project{ID: projID2})
		require.NoError(t, err)

		invite := &console.ProjectInvitation{
			ProjectID: projID,
			Email:     email,
			InviterID: &inviterID,
		}
		inviteSameEmail := &console.ProjectInvitation{
			ProjectID: projID2,
			Email:     email,
		}
		inviteSameProject := &console.ProjectInvitation{
			ProjectID: projID,
			Email:     email2,
		}

		if !t.Run("insert invitations", func(t *testing.T) {
			// Expect failure because no user with inviterID exists.
			_, err = invitesDB.Insert(ctx, invite)
			require.Error(t, err)

			_, err = db.Console().Users().Insert(ctx, &console.User{
				ID:           inviterID,
				PasswordHash: testrand.Bytes(8),
			})
			require.NoError(t, err)

			invite, err = invitesDB.Insert(ctx, invite)
			require.NoError(t, err)
			require.WithinDuration(t, time.Now(), invite.CreatedAt, time.Minute)
			require.Equal(t, projID, invite.ProjectID)
			require.Equal(t, strings.ToUpper(email), invite.Email)

			// Duplicate invitations should be rejected.
			_, err = invitesDB.Insert(ctx, invite)
			require.Error(t, err)

			inviteSameEmail, err = invitesDB.Insert(ctx, inviteSameEmail)
			require.NoError(t, err)
			inviteSameProject, err = invitesDB.Insert(ctx, inviteSameProject)
			require.NoError(t, err)
		}) {
			// None of the following subtests will pass if invitation insertion failed.
			return
		}

		t.Run("get invitation", func(t *testing.T) {
			ctx := testcontext.New(t)

			other, err := invitesDB.Get(ctx, projID, "nobody@mail.test")
			require.ErrorIs(t, err, sql.ErrNoRows)
			require.Nil(t, other)

			other, err = invitesDB.Get(ctx, projID, email)
			require.NoError(t, err)
			require.Equal(t, invite, other)
		})

		t.Run("get invitations by email", func(t *testing.T) {
			ctx := testcontext.New(t)

			invites, err := invitesDB.GetByEmail(ctx, "nobody@mail.test")
			require.NoError(t, err)
			require.Empty(t, invites)

			invites, err = invitesDB.GetByEmail(ctx, "uSeR@mAiL.tEsT")
			require.NoError(t, err)
			require.ElementsMatch(t, invites, []console.ProjectInvitation{*invite, *inviteSameEmail})
		})

		t.Run("get invitations by project ID", func(t *testing.T) {
			ctx := testcontext.New(t)

			invites, err := invitesDB.GetByProjectID(ctx, testrand.UUID())
			require.NoError(t, err)
			require.Empty(t, invites)

			invites, err = invitesDB.GetByProjectID(ctx, projID)
			require.NoError(t, err)
			require.ElementsMatch(t, invites, []console.ProjectInvitation{*invite, *inviteSameProject})
		})

		t.Run("ensure inviter removal nullifies inviter ID", func(t *testing.T) {
			ctx := testcontext.New(t)

			require.NoError(t, db.Console().Users().Delete(ctx, inviterID))
			invite, err := invitesDB.Get(ctx, projID, email)
			require.NoError(t, err)
			require.Nil(t, invite.InviterID)
		})

		t.Run("delete invitation", func(t *testing.T) {
			ctx := testcontext.New(t)

			require.NoError(t, invitesDB.Delete(ctx, projID, email))

			invites, err := invitesDB.GetByEmail(ctx, email)
			require.NoError(t, err)
			require.Equal(t, invites, []console.ProjectInvitation{*inviteSameEmail})
		})

		t.Run("ensure project removal deletes invitations", func(t *testing.T) {
			ctx := testcontext.New(t)

			require.NoError(t, projectsDB.Delete(ctx, projID))

			invites, err := invitesDB.GetByProjectID(ctx, projID)
			require.NoError(t, err)
			require.Empty(t, invites)

			invites, err = invitesDB.GetByEmail(ctx, email)
			require.NoError(t, err)
			require.Equal(t, invites, []console.ProjectInvitation{*inviteSameEmail})
		})
	})
}

func TestDeleteBefore(t *testing.T) {
	maxAge := time.Hour
	now := time.Now()
	expiration := now.Add(-maxAge)

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		invitesDB := db.Console().ProjectInvitations()
		now := time.Now()

		// Only positive page sizes should be allowed.
		require.Error(t, invitesDB.DeleteBefore(ctx, time.Time{}, 0, 0))
		require.Error(t, invitesDB.DeleteBefore(ctx, time.Time{}, 0, -1))

		createInvite := func(createdAt time.Time) *console.ProjectInvitation {
			projID := testrand.UUID()
			_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projID})
			require.NoError(t, err)

			invite, err := invitesDB.Insert(ctx, &console.ProjectInvitation{ProjectID: projID})
			require.NoError(t, err)

			result, err := db.Testing().RawDB().ExecContext(ctx,
				"UPDATE project_invitations SET created_at = $1 WHERE project_id = $2",
				createdAt, invite.ProjectID,
			)
			require.NoError(t, err)

			count, err := result.RowsAffected()
			require.NoError(t, err)
			require.EqualValues(t, 1, count)

			return invite
		}

		newInvite := createInvite(now)
		oldInvite := createInvite(expiration.Add(-time.Second))

		require.NoError(t, invitesDB.DeleteBefore(ctx, expiration, 0, 1))

		// Ensure that the old invitation record was deleted and the other remains.
		_, err := invitesDB.Get(ctx, oldInvite.ProjectID, oldInvite.Email)
		require.ErrorIs(t, err, sql.ErrNoRows)
		_, err = invitesDB.Get(ctx, newInvite.ProjectID, newInvite.Email)
		require.NoError(t, err)
	})
}
