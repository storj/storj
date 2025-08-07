// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestGetPagedWithInvitationsByProjectID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		membersDB := db.Console().ProjectMembers()

		projectID := testrand.UUID()
		_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
		require.NoError(t, err)

		memberUser, err := db.Console().Users().Insert(ctx, &console.User{
			FullName:     "Alice",
			Email:        "alice@mail.test",
			ID:           testrand.UUID(),
			PasswordHash: testrand.Bytes(8),
		})
		require.NoError(t, err)
		_, err = db.Console().ProjectMembers().Insert(ctx, memberUser.ID, projectID, console.RoleAdmin)
		require.NoError(t, err)

		_, err = db.Console().ProjectInvitations().Upsert(ctx, &console.ProjectInvitation{
			ProjectID: projectID,
			Email:     "bob@mail.test",
		})
		require.NoError(t, err)

		t.Run("paging", func(t *testing.T) {
			ctx := testcontext.New(t)

			for _, tt := range []struct {
				limit         uint
				page          uint
				expectedCount int
			}{
				{limit: 2, page: 1, expectedCount: 2},
				{limit: 1, page: 1, expectedCount: 1},
				{limit: 1, page: 2, expectedCount: 1},
			} {
				cursor := console.ProjectMembersCursor{Limit: tt.limit, Page: tt.page}
				page, err := membersDB.GetPagedWithInvitationsByProjectID(ctx, projectID, cursor)
				require.NoError(t, err)
				require.Equal(t, tt.expectedCount, len(page.ProjectInvitations)+len(page.ProjectMembers),
					fmt.Sprintf("error occurred with limit %d, page %d", tt.limit, tt.page))
			}

			_, err = membersDB.GetPagedWithInvitationsByProjectID(ctx, projectID, console.ProjectMembersCursor{Limit: 1, Page: 3})
			require.Error(t, err)
		})

		t.Run("search", func(t *testing.T) {
			ctx := testcontext.New(t)

			for _, tt := range []struct {
				search        string
				expectMembers bool
				expectInvites bool
			}{
				{search: "aLiCe", expectMembers: true},
				{search: "@ test", expectMembers: true, expectInvites: true},
				{search: "bad"},
			} {
				errMsg := "unexpected result for search '" + tt.search + "'"

				cursor := console.ProjectMembersCursor{Search: tt.search, Limit: 2, Page: 1}
				page, err := membersDB.GetPagedWithInvitationsByProjectID(ctx, projectID, cursor)
				require.NoError(t, err, errMsg)

				if tt.expectMembers {
					require.NotEmpty(t, page.ProjectMembers, errMsg)
				} else {
					require.Empty(t, page.ProjectMembers, errMsg)
				}

				if tt.expectInvites {
					require.NotEmpty(t, page.ProjectInvitations, errMsg)
				} else {
					require.Empty(t, page.ProjectInvitations, errMsg)
				}
			}
		})

		t.Run("ordering", func(t *testing.T) {
			ctx := testcontext.New(t)

			projectID := testrand.UUID()
			_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
			require.NoError(t, err)

			var memberIDs []uuid.UUID
			for i := 0; i < 3; i++ {
				id := uuid.UUID{}
				id[len(id)-1] = byte(i + 1)
				memberIDs = append(memberIDs, id)

				user := console.User{
					FullName:     strconv.Itoa(i),
					Email:        fmt.Sprintf("%d@mail.test", (i+2)%3),
					ID:           id,
					PasswordHash: testrand.Bytes(8),
				}

				_, err := db.Console().Users().Insert(ctx, &user)
				require.NoError(t, err)

				_, err = db.Console().ProjectMembers().Insert(ctx, user.ID, projectID, console.RoleAdmin)
				require.NoError(t, err)

				result, err := db.Testing().RawDB().ExecContext(ctx,
					db.Testing().Rebind("UPDATE project_members SET created_at = ? WHERE member_id = ?"),
					time.Time{}.Add(time.Duration((i+1)%3)*time.Hour), id,
				)
				require.NoError(t, err)

				count, err := result.RowsAffected()
				require.NoError(t, err)
				require.EqualValues(t, 1, count)
			}

			totalCount, err := db.Console().ProjectMembers().GetTotalCountByProjectID(ctx, projectID)
			require.NoError(t, err)
			require.EqualValues(t, 3, totalCount)

			for _, tt := range []struct {
				order     console.ProjectMemberOrder
				memberIDs []uuid.UUID
			}{
				{
					order:     console.Name,
					memberIDs: []uuid.UUID{memberIDs[0], memberIDs[1], memberIDs[2]},
				}, {
					order:     console.Email,
					memberIDs: []uuid.UUID{memberIDs[1], memberIDs[2], memberIDs[0]},
				}, {
					order:     console.Created,
					memberIDs: []uuid.UUID{memberIDs[2], memberIDs[0], memberIDs[1]},
				},
			} {
				errMsg := func(cursor console.ProjectMembersCursor) string {
					return fmt.Sprintf("unexpected result when ordering by %s, %s",
						[]string{"name", "email", "creation date"}[cursor.Order-1],
						[]string{"ascending", "descending"}[cursor.OrderDirection-1])
				}

				getIDsFromDB := func(cursor console.ProjectMembersCursor) (ids []uuid.UUID) {
					page, err := membersDB.GetPagedWithInvitationsByProjectID(ctx, projectID, cursor)
					require.NoError(t, err, errMsg(cursor))
					for _, member := range page.ProjectMembers {
						ids = append(ids, member.MemberID)
					}
					return ids
				}

				cursor := console.ProjectMembersCursor{
					Limit: uint(len(tt.memberIDs)),
					Page:  1, Order: tt.order,
					OrderDirection: console.Ascending,
				}
				require.Equal(t, tt.memberIDs, getIDsFromDB(cursor), errMsg(cursor))

				cursor.OrderDirection = console.Descending
				var reverseMemberIDs []uuid.UUID
				for i := len(tt.memberIDs) - 1; i >= 0; i-- {
					reverseMemberIDs = append(reverseMemberIDs, tt.memberIDs[i])
				}
				require.Equal(t, reverseMemberIDs, getIDsFromDB(cursor), errMsg(cursor))
			}
		})
	})
}

func TestUpdateRole(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		membersDB := db.Console().ProjectMembers()

		projectID := testrand.UUID()
		_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
		require.NoError(t, err)

		memberUser, err := db.Console().Users().Insert(ctx, &console.User{
			FullName:     "Alice",
			Email:        "alice@mail.test",
			ID:           testrand.UUID(),
			PasswordHash: testrand.Bytes(8),
		})
		require.NoError(t, err)

		member, err := membersDB.Insert(ctx, memberUser.ID, projectID, console.RoleAdmin)
		require.NoError(t, err)
		require.Equal(t, console.RoleAdmin, member.Role)

		member, err = membersDB.UpdateRole(ctx, memberUser.ID, projectID, console.RoleMember)
		require.NoError(t, err)
		require.Equal(t, console.RoleMember, member.Role)
	})
}
