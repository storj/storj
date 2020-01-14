// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProjectMembersRepository(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		// repositories
		users := db.Console().Users()
		projects := db.Console().Projects()
		projectMembers := db.Console().ProjectMembers()

		createdUsers, createdProjects := prepareUsersAndProjects(ctx, t, users, projects)

		t.Run("Can't insert projectMember without memberID", func(t *testing.T) {
			missingUserID := testrand.UUID()

			projMember, err := projectMembers.Insert(ctx, missingUserID, createdProjects[0].ID)
			assert.Nil(t, projMember)
			assert.Error(t, err)
		})

		t.Run("Can't insert projectMember without projectID", func(t *testing.T) {
			missingProjectID := testrand.UUID()

			projMember, err := projectMembers.Insert(ctx, createdUsers[0].ID, missingProjectID)
			assert.Nil(t, projMember)
			assert.Error(t, err)
		})

		t.Run("Insert  success", func(t *testing.T) {
			projMember1, err := projectMembers.Insert(ctx, createdUsers[0].ID, createdProjects[0].ID)
			assert.NotNil(t, projMember1)
			assert.NoError(t, err)

			projMember2, err := projectMembers.Insert(ctx, createdUsers[1].ID, createdProjects[0].ID)
			assert.NotNil(t, projMember2)
			assert.NoError(t, err)

			projMember3, err := projectMembers.Insert(ctx, createdUsers[3].ID, createdProjects[0].ID)
			assert.NotNil(t, projMember3)
			assert.NoError(t, err)

			projMember4, err := projectMembers.Insert(ctx, createdUsers[4].ID, createdProjects[0].ID)
			assert.NotNil(t, projMember4)
			assert.NoError(t, err)

			projMember5, err := projectMembers.Insert(ctx, createdUsers[5].ID, createdProjects[0].ID)
			assert.NotNil(t, projMember5)
			assert.NoError(t, err)

			projMember6, err := projectMembers.Insert(ctx, createdUsers[2].ID, createdProjects[1].ID)
			assert.NotNil(t, projMember6)
			assert.NoError(t, err)

			projMember7, err := projectMembers.Insert(ctx, createdUsers[0].ID, createdProjects[1].ID)
			assert.NotNil(t, projMember7)
			assert.NoError(t, err)
		})

		t.Run("Get projects by userID", func(t *testing.T) {
			projects, err := projects.GetByUserID(ctx, createdUsers[0].ID)
			assert.NoError(t, err)
			assert.NotNil(t, projects)
			assert.Equal(t, len(projects), 2)
		})

		t.Run("Get paged", func(t *testing.T) {
			// sql injection test. F.E '%SomeText%' = > ''%SomeText%' OR 'x' != '%'' will be true
			members, err := projectMembers.GetPagedByProjectID(ctx, createdProjects[0].ID, console.ProjectMembersCursor{Limit: 6, Search: "son%' OR 'x' != '", Order: 2, Page: 1})
			assert.NoError(t, err)
			assert.NotNil(t, members)
			assert.Equal(t, uint64(0), members.TotalCount)
			assert.Equal(t, uint(0), members.CurrentPage)
			assert.Equal(t, uint(0), members.PageCount)
			assert.Equal(t, 0, len(members.ProjectMembers))

			members, err = projectMembers.GetPagedByProjectID(ctx, createdProjects[0].ID, console.ProjectMembersCursor{Limit: 3, Search: "", Order: 1, Page: 1})
			assert.NoError(t, err)
			assert.NotNil(t, members)
			assert.Equal(t, uint64(5), members.TotalCount)
			assert.Equal(t, uint(1), members.CurrentPage)
			assert.Equal(t, uint(2), members.PageCount)
			assert.Equal(t, 3, len(members.ProjectMembers))

			members, err = projectMembers.GetPagedByProjectID(ctx, createdProjects[0].ID, console.ProjectMembersCursor{Limit: 2, Search: "iam", Order: 2, Page: 1}) // TODO: fix case sensitity issues and change back to "Liam"
			assert.NoError(t, err)
			assert.NotNil(t, members)
			assert.Equal(t, uint64(2), members.TotalCount)
			assert.Equal(t, 2, len(members.ProjectMembers))

			members, err = projectMembers.GetPagedByProjectID(ctx, createdProjects[0].ID, console.ProjectMembersCursor{Limit: 2, Search: "iam", Order: 1, Page: 1}) // TODO: fix case sensitity issues and change back to "Liam"
			assert.NoError(t, err)
			assert.NotNil(t, members)
			assert.Equal(t, uint64(2), members.TotalCount)
			assert.Equal(t, 2, len(members.ProjectMembers))

			members, err = projectMembers.GetPagedByProjectID(ctx, createdProjects[0].ID, console.ProjectMembersCursor{Limit: 6, Search: "son", Order: 123, Page: 1})
			assert.NoError(t, err)
			assert.NotNil(t, members)
			assert.Equal(t, uint64(5), members.TotalCount)
			assert.Equal(t, 5, len(members.ProjectMembers))

			members, err = projectMembers.GetPagedByProjectID(ctx, createdProjects[0].ID, console.ProjectMembersCursor{Limit: 6, Search: "son", Order: 2, Page: 1})
			assert.NoError(t, err)
			assert.NotNil(t, members)
			assert.Equal(t, uint64(5), members.TotalCount)
			assert.Equal(t, 5, len(members.ProjectMembers))
		})

		t.Run("Get member by memberID success", func(t *testing.T) {
			originalMember1 := createdUsers[0]
			selectedMembers1, err := projectMembers.GetByMemberID(ctx, originalMember1.ID)

			assert.NotNil(t, selectedMembers1)
			assert.NoError(t, err)
			assert.Equal(t, originalMember1.ID, selectedMembers1[0].MemberID)

			originalMember2 := createdUsers[1]
			selectedMembers2, err := projectMembers.GetByMemberID(ctx, originalMember2.ID)

			assert.NotNil(t, selectedMembers2)
			assert.NoError(t, err)
			assert.Equal(t, originalMember2.ID, selectedMembers2[0].MemberID)
		})

		t.Run("Delete member by memberID and projectID success", func(t *testing.T) {
			err := projectMembers.Delete(ctx, createdUsers[0].ID, createdProjects[0].ID)
			assert.NoError(t, err)

			projMembers, err := projectMembers.GetPagedByProjectID(ctx, createdProjects[0].ID, console.ProjectMembersCursor{
				Order:  1,
				Search: "",
				Limit:  100,
				Page:   1,
			})
			assert.NoError(t, err)
			assert.NotNil(t, projectMembers)
			assert.Equal(t, len(projMembers.ProjectMembers), 4)
		})
	})
}

func prepareUsersAndProjects(ctx context.Context, t *testing.T, users console.Users, projects console.Projects) ([]*console.User, []*console.Project) {
	usersList := []*console.User{{
		ID:           testrand.UUID(),
		Email:        "2email2@mail.test",
		PasswordHash: []byte("some_readable_hash"),
		ShortName:    "Liam",
		FullName:     "Liam Jameson",
	}, {
		ID:           testrand.UUID(),
		Email:        "1email1@mail.test",
		PasswordHash: []byte("some_readable_hash"),
		ShortName:    "William",
		FullName:     "Noahson William",
	}, {
		ID:           testrand.UUID(),
		Email:        "email3@mail.test",
		PasswordHash: []byte("some_readable_hash"),
		ShortName:    "Mason",
		FullName:     "Mason Elijahson",
	}, {
		ID:           testrand.UUID(),
		Email:        "email4@mail.test",
		PasswordHash: []byte("some_readable_hash"),
		ShortName:    "Oliver",
		FullName:     "Oliver Jacobson",
	}, {
		ID:           testrand.UUID(),
		Email:        "email5@mail.test",
		PasswordHash: []byte("some_readable_hash"),
		ShortName:    "Lucas",
		FullName:     "Michaelson Lucas",
	}, {
		ID:           testrand.UUID(),
		Email:        "email6@mail.test",
		PasswordHash: []byte("some_readable_hash"),
		ShortName:    "Alexander",
		FullName:     "Alexander Ethanson",
	},
	}

	var err error
	for i, user := range usersList {
		usersList[i], err = users.Insert(ctx, user)
		require.NoError(t, err)
	}

	projectList := []*console.Project{
		{
			Name:        "projName1",
			Description: "Test project 1",
		},
		{
			Name:        "projName2",
			Description: "Test project 1",
		},
	}

	for i, project := range projectList {
		projectList[i], err = projects.Insert(ctx, project)
		require.NoError(t, err)
	}

	return usersList, projectList
}
