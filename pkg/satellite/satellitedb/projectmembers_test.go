// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/satellite"
)

func TestProjectMembersRepository(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// creating in-memory db and opening connection
	db, err := New("sqlite3", "file::memory:?mode=memory&cache=shared")
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
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Can't insert projectMember without projectID", func(t *testing.T) {
		unexistingProjectID, err := uuid.New()
		assert.NoError(t, err)

		projMember, err := projectMembers.Insert(ctx, createdUsers[0].ID, *unexistingProjectID)
		assert.Nil(t, projMember)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Insert  success", func(t *testing.T) {
		projMember1, err := projectMembers.Insert(ctx, createdUsers[0].ID, createdProjects[0].ID)
		assert.NotNil(t, projMember1)
		assert.Nil(t, err)
		assert.NoError(t, err)

		projMember2, err := projectMembers.Insert(ctx, createdUsers[1].ID, createdProjects[0].ID)
		assert.NotNil(t, projMember2)
		assert.Nil(t, err)
		assert.NoError(t, err)

		projMember3, err := projectMembers.Insert(ctx, createdUsers[2].ID, createdProjects[1].ID)
		assert.NotNil(t, projMember3)
		assert.Nil(t, err)
		assert.NoError(t, err)
	})

	t.Run("Get member by memberID success", func(t *testing.T) {
		originalMember1 := createdUsers[0]
		selectedMember1, err := projectMembers.GetByMemberID(ctx, originalMember1.ID)

		assert.NotNil(t, selectedMember1)
		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.Equal(t, originalMember1.ID, selectedMember1.MemberID)

		originalMember2 := createdUsers[1]
		selectedMember2, err := projectMembers.GetByMemberID(ctx, originalMember2.ID)

		assert.NotNil(t, selectedMember2)
		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.Equal(t, originalMember2.ID, selectedMember2.MemberID)
	})

	t.Run("Get member by projectID success", func(t *testing.T) {
		originalProject1 := createdProjects[0]
		projectMembers1, err := projectMembers.GetByProjectID(ctx, originalProject1.ID)

		assert.NotNil(t, projectMembers1)
		assert.Equal(t, 2, len(projectMembers1))
		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.Equal(t, projectMembers1[0].MemberID, createdUsers[0].ID)
		assert.Equal(t, projectMembers1[1].MemberID, createdUsers[1].ID)

		originalProject2 := createdProjects[1]
		projectMembers2, err := projectMembers.GetByProjectID(ctx, originalProject2.ID)

		assert.NotNil(t, projectMembers2)
		assert.Equal(t, 1, len(projectMembers2))
		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.Equal(t, projectMembers2[0].MemberID, createdUsers[2].ID)
	})

	t.Run("Get all and get by id success", func(t *testing.T) {
		allProjMembers, err := projectMembers.GetAll(ctx)
		assert.NotNil(t, allProjMembers)
		assert.Equal(t, 3, len(allProjMembers))
		assert.Nil(t, err)
		assert.NoError(t, err)

		projMember1, err := projectMembers.Get(ctx, allProjMembers[0].ID)
		assert.NotNil(t, projMember1)
		assert.Nil(t, err)
		assert.NoError(t, err)

		projMember2, err := projectMembers.Get(ctx, allProjMembers[1].ID)
		assert.NotNil(t, projMember2)
		assert.Nil(t, err)
		assert.NoError(t, err)

		projMember3, err := projectMembers.Get(ctx, allProjMembers[2].ID)
		assert.NotNil(t, projMember3)
		assert.Nil(t, err)
		assert.NoError(t, err)
	})

	t.Run("Update success", func(t *testing.T) {
		// fetching member of project #2
		members, err := projectMembers.GetByProjectID(ctx, createdProjects[1].ID)
		assert.NotNil(t, members)
		assert.Equal(t, 1, len(members))
		assert.Nil(t, err)
		assert.NoError(t, err)

		// set its proj id to proj1 id
		projMemberToUpdate := members[0]
		projMemberToUpdate.ProjectID = createdProjects[0].ID

		err = projectMembers.Update(ctx, &projMemberToUpdate)
		assert.Nil(t, err)
		assert.NoError(t, err)

		// checking that proj 2 has 0 members
		members, err = projectMembers.GetByProjectID(ctx, createdProjects[1].ID)
		assert.Equal(t, 0, len(members))
		assert.Nil(t, members)
		assert.Nil(t, err)
		assert.NoError(t, err)

		// checking that proj 1 has 3 members after update
		members, err = projectMembers.GetByProjectID(ctx, createdProjects[0].ID)
		assert.NotNil(t, members)
		assert.Equal(t, 3, len(members))
		assert.Nil(t, err)
		assert.NoError(t, err)
	})

	t.Run("Delete success", func(t *testing.T) {
		members, err := projectMembers.GetByProjectID(ctx, createdProjects[0].ID)
		assert.NotNil(t, members)
		assert.Equal(t, 3, len(members))
		assert.Nil(t, err)
		assert.NoError(t, err)

		err = projectMembers.Delete(ctx, members[2].ID)
		assert.Nil(t, err)
		assert.NoError(t, err)

		members, err = projectMembers.GetByProjectID(ctx, createdProjects[0].ID)
		assert.NotNil(t, members)
		assert.Equal(t, 2, len(members))
		assert.Nil(t, err)
		assert.NoError(t, err)
	})
}

func prepareUsersAndProjects(ctx context.Context, t *testing.T, users satellite.Users, projects satellite.Projects) ([]*satellite.User, []*satellite.Project) {
	usersList := []*satellite.User{{
		Email:        "email1@ukr.net",
		PasswordHash: []byte("some_readable_hash"),
		LastName:     "LastName",
		FirstName:    "FirstName",
	}, {
		Email:        "email2@ukr.net",
		PasswordHash: []byte("some_readable_hash"),
		LastName:     "LastName",
		FirstName:    "FirstName",
	}, {
		Email:        "email3@ukr.net",
		PasswordHash: []byte("some_readable_hash"),
		LastName:     "LastName",
		FirstName:    "FirstName",
	},
	}

	var err error
	for i, user := range usersList {
		usersList[i], err = users.Insert(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
	}

	projectList := []*satellite.Project{
		{
			Name:          "projName1",
			TermsAccepted: 1,
			Description:   "Test project 1",
			OwnerID:       &usersList[0].ID,
		},
		{
			Name:          "projName2",
			TermsAccepted: 1,
			Description:   "Test project 1",
			OwnerID:       &usersList[1].ID,
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
