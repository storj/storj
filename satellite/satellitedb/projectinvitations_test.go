// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
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
		projID := testrand.UUID()
		projID2 := testrand.UUID()
		email := "user@mail.test"
		email2 := "user2@mail.test"

		_, err := projectsDB.Insert(ctx, &console.Project{ID: projID})
		require.NoError(t, err)
		_, err = projectsDB.Insert(ctx, &console.Project{ID: projID2})
		require.NoError(t, err)

		invite, err := invitesDB.Insert(ctx, projID, email)
		require.NoError(t, err)
		require.WithinDuration(t, time.Now(), invite.CreatedAt, time.Minute)
		require.Equal(t, projID, invite.ProjectID)
		require.Equal(t, strings.ToUpper(email), invite.Email)

		_, err = invitesDB.Insert(ctx, projID, email)
		require.Error(t, err)

		inviteSameEmail, err := invitesDB.Insert(ctx, projID2, email)
		require.NoError(t, err)
		inviteSameProject, err := invitesDB.Insert(ctx, projID, email2)
		require.NoError(t, err)

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
			id := testrand.UUID()
			_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: id})
			require.NoError(t, err)
			invite, err := invitesDB.Insert(ctx, id, "")
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
		invites, err := invitesDB.GetByProjectID(ctx, oldInvite.ProjectID)
		require.NoError(t, err)
		require.Empty(t, invites)

		invites, err = invitesDB.GetByProjectID(ctx, newInvite.ProjectID)
		require.NoError(t, err)
		require.Len(t, invites, 1)
	})
}
