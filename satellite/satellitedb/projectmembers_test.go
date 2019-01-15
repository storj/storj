// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite/console"
)

func TestProjectMembersRepository(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// creating in-memory db and opening connection
	db, err := NewConsoleDB("sqlite3", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(db.Close)

	// creating tables
	err = db.CreateTables()
	if err != nil {
		t.Fatal(err)
	}

	// repositories
	users := db.Users()
	projects := db.Projects()
	projectMembers := db.ProjectMembers()

	createdUsers, createdProjects := prepareUsersAndProjects(ctx, t, users, projects)

	t.Run("Can't insert projectMember without memberID", func(t *testing.T) {
		unexistingUserID, err := uuid.New()
		assert.NoError(t, err)

		projMember, err := projectMembers.Insert(ctx, *unexistingUserID, createdProjects[0].ID)
		assert.Nil(t, projMember)
		assert.Error(t, err)
	})

	t.Run("Can't insert projectMember without projectID", func(t *testing.T) {
		unexistingProjectID, err := uuid.New()
		assert.NoError(t, err)

		projMember, err := projectMembers.Insert(ctx, createdUsers[0].ID, *unexistingProjectID)
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
		members, err := projectMembers.GetByProjectID(ctx, createdProjects[0].ID, console.Pagination{Limit: 6, Offset: 0, Search: "son%' OR 'x' != '", Order: 2})
		assert.NoError(t, err)
		assert.Nil(t, members)
		assert.Equal(t, 0, len(members))

		members, err = projectMembers.GetByProjectID(ctx, createdProjects[0].ID, console.Pagination{Limit: 3, Offset: 0, Search: "", Order: 1})
		assert.NoError(t, err)
		assert.NotNil(t, members)
		assert.Equal(t, 3, len(members))

		members, err = projectMembers.GetByProjectID(ctx, createdProjects[0].ID, console.Pagination{Limit: 2, Offset: 0, Search: "Liam", Order: 2})
		assert.NoError(t, err)
		assert.NotNil(t, members)
		assert.Equal(t, 2, len(members))

		members, err = projectMembers.GetByProjectID(ctx, createdProjects[0].ID, console.Pagination{Limit: 2, Offset: 0, Search: "Liam", Order: 1})
		assert.NoError(t, err)
		assert.NotNil(t, members)
		assert.Equal(t, 2, len(members))

		members, err = projectMembers.GetByProjectID(ctx, createdProjects[0].ID, console.Pagination{Limit: 6, Offset: 0, Search: "son", Order: 123})
		assert.NoError(t, err)
		assert.NotNil(t, members)
		assert.Equal(t, 5, len(members))

		members, err = projectMembers.GetByProjectID(ctx, createdProjects[0].ID, console.Pagination{Limit: 6, Offset: 3, Search: "son", Order: 2})
		assert.NoError(t, err)
		assert.NotNil(t, members)
		assert.Equal(t, 2, len(members))

		members, err = projectMembers.GetByProjectID(ctx, createdProjects[0].ID, console.Pagination{Limit: -123, Offset: -14, Search: "son", Order: 2})
		assert.Error(t, err)
		assert.Nil(t, members)
		assert.Equal(t, 0, len(members))
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

		projMembers, err := projectMembers.GetByProjectID(ctx, createdProjects[0].ID, console.Pagination{
			Order:  1,
			Search: "",
			Offset: 0,
			Limit:  100,
		})
		assert.NoError(t, err)
		assert.NotNil(t, projectMembers)
		assert.Equal(t, len(projMembers), 4)
	})
}

func prepareUsersAndProjects(ctx context.Context, t *testing.T, users console.Users, projects console.Projects) ([]*console.User, []*console.Project) {
	usersList := []*console.User{{
		Email:        "2email2@ukr.net",
		PasswordHash: []byte("some_readable_hash"),
		LastName:     "Liam",
		FirstName:    "Jameson",
	}, {
		Email:        "1email1@ukr.net",
		PasswordHash: []byte("some_readable_hash"),
		LastName:     "William",
		FirstName:    "Noahson",
	}, {
		Email:        "email3@ukr.net",
		PasswordHash: []byte("some_readable_hash"),
		LastName:     "Mason",
		FirstName:    "Elijahson",
	}, {
		Email:        "email4@ukr.net",
		PasswordHash: []byte("some_readable_hash"),
		LastName:     "Oliver",
		FirstName:    "Jacobson",
	}, {
		Email:        "email5@ukr.net",
		PasswordHash: []byte("some_readable_hash"),
		LastName:     "Lucas",
		FirstName:    "Michaelson",
	}, {
		Email:        "email6@ukr.net",
		PasswordHash: []byte("some_readable_hash"),
		LastName:     "Alexander",
		FirstName:    "Ethanson",
	},
	}

	var err error
	for i, user := range usersList {
		usersList[i], err = users.Insert(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
	}

	projectList := []*console.Project{
		{
			Name:          "projName1",
			TermsAccepted: 1,
			Description:   "Test project 1",
		},
		{
			Name:          "projName2",
			TermsAccepted: 1,
			Description:   "Test project 1",
		},
	}

	for i, project := range projectList {
		projectList[i], err = projects.Insert(ctx, project)
		if err != nil {
			t.Fatal(err)
		}
	}

	return usersList, projectList
}

func TestSanitizedOrderColumnName(t *testing.T) {
	testCases := [...]struct {
		orderNumber int8
		orderColumn string
	}{
		0: {0, "u.first_name"},
		1: {1, "u.first_name"},
		2: {2, "u.email"},
		3: {3, "u.created_at"},
		4: {4, "u.first_name"},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.orderColumn, sanitizedOrderColumnName(console.ProjectMemberOrder(tc.orderNumber)))
	}
}
