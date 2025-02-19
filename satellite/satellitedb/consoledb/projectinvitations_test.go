// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

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
			_, err = invitesDB.Upsert(ctx, invite)
			require.Error(t, err)

			_, err = db.Console().Users().Insert(ctx, &console.User{
				ID:           inviterID,
				PasswordHash: testrand.Bytes(8),
			})
			require.NoError(t, err)

			invite, err = invitesDB.Upsert(ctx, invite)
			require.NoError(t, err)
			require.WithinDuration(t, time.Now(), invite.CreatedAt, time.Minute)
			require.Equal(t, projID, invite.ProjectID)
			require.Equal(t, strings.ToUpper(email), invite.Email)

			inviteSameEmail, err = invitesDB.Upsert(ctx, inviteSameEmail)
			require.NoError(t, err)
			inviteSameProject, err = invitesDB.Upsert(ctx, inviteSameProject)
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

		t.Run("get invitations by email for active projects", func(t *testing.T) {
			ctx := testcontext.New(t)

			activeProjID := testrand.UUID()
			disabledProjID := testrand.UUID()
			email3 := "user3@mail.test"

			_, err = projectsDB.Insert(ctx, &console.Project{ID: activeProjID})
			require.NoError(t, err)
			_, err = projectsDB.Insert(ctx, &console.Project{ID: disabledProjID})
			require.NoError(t, err)

			inviteToActiveProj := &console.ProjectInvitation{
				ProjectID: activeProjID,
				Email:     email3,
			}
			inviteToDisabledProj := &console.ProjectInvitation{
				ProjectID: disabledProjID,
				Email:     email3,
			}

			inviteToActiveProj, err = invitesDB.Upsert(ctx, inviteToActiveProj)
			require.NoError(t, err)
			inviteToDisabledProj, err = invitesDB.Upsert(ctx, inviteToDisabledProj)
			require.NoError(t, err)

			invites, err := invitesDB.GetForActiveProjectsByEmail(ctx, email3)
			require.NoError(t, err)
			require.ElementsMatch(t, []console.ProjectInvitation{*inviteToActiveProj, *inviteToDisabledProj}, invites)

			err = projectsDB.UpdateStatus(ctx, disabledProjID, console.ProjectDisabled)
			require.NoError(t, err)

			invites, err = invitesDB.GetForActiveProjectsByEmail(ctx, email3)
			require.NoError(t, err)
			require.ElementsMatch(t, []console.ProjectInvitation{*inviteToActiveProj}, invites)
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

		t.Run("ensure inviter removal removes the invite", func(t *testing.T) {
			ctx := testcontext.New(t)
			_, err := invitesDB.Get(ctx, projID, email)
			require.NoError(t, err)
			require.NoError(t, db.Console().Users().Delete(ctx, inviterID))
			_, err = invitesDB.Get(ctx, projID, email)
			require.ErrorIs(t, err, sql.ErrNoRows)
		})

		t.Run("update invitation", func(t *testing.T) {
			ctx := testcontext.New(t)

			inviter, err := db.Console().Users().Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				PasswordHash: testrand.Bytes(8),
			})
			require.NoError(t, err)
			invite.InviterID = &inviter.ID

			oldCreatedAt := invite.CreatedAt

			invite, err = invitesDB.Upsert(ctx, invite)
			require.NoError(t, err)
			require.Equal(t, inviter.ID, *invite.InviterID)
			require.True(t, invite.CreatedAt.After(oldCreatedAt))
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
