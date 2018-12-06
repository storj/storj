// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/satellite"
	"storj.io/storj/pkg/satellite/satellitedb/dbx"
)

func TestProjectsRepository(t *testing.T) {
	//testing constants
	const (
		// for user
		lastName = "lastName"
		email    = "email@ukr.net"
		pass     = "123456"
		userName = "name"

		// for project
		name        = "Project"
		companyName = "Comp Name"
		description = "some description"

		// updated project values
		newDescription = "some new description"
	)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// creating in-memory db and opening connection
	// to test with real db3 file use this connection string - "../db/accountdb.db3"
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

	var owner *satellite.User

	t.Run("Can insert project without owner", func(t *testing.T) {
		project := &satellite.Project{
			OwnerID:       nil,
			Name:          name,
			CompanyName:   companyName,
			Description:   description,
			TermsAccepted: 1,
		}

		createdProject, err := projects.Insert(ctx, project)

		assert.NotNil(t, createdProject)
		assert.Nil(t, err)
		assert.NoError(t, err)
	})

	t.Run("Insert project successfully", func(t *testing.T) {
		owner, err = users.Insert(ctx, &satellite.User{
			FirstName:    userName,
			LastName:     lastName,
			Email:        email,
			PasswordHash: []byte(pass),
		})

		assert.NoError(t, err)
		assert.NotNil(t, owner)

		project := &satellite.Project{
			OwnerID: &owner.ID,

			Name:          name,
			CompanyName:   companyName,
			Description:   description,
			TermsAccepted: 1,
		}

		createdProject, err := projects.Insert(ctx, project)

		assert.NotNil(t, createdProject)
		assert.Nil(t, err)
		assert.NoError(t, err)
	})

	t.Run("Get project success", func(t *testing.T) {
		projectsByOwnerID, err := projects.GetByOwnerID(ctx, owner.ID)

		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.Equal(t, len(projectsByOwnerID), 1)

		assert.Equal(t, projectsByOwnerID[0].OwnerID, &owner.ID)
		assert.Equal(t, projectsByOwnerID[0].Name, name)
		assert.Equal(t, projectsByOwnerID[0].CompanyName, companyName)
		assert.Equal(t, projectsByOwnerID[0].Description, description)
		assert.Equal(t, projectsByOwnerID[0].TermsAccepted, 1)

		projectByID, err := projects.Get(ctx, projectsByOwnerID[0].ID)

		assert.Nil(t, err)
		assert.NoError(t, err)

		assert.Equal(t, projectByID.ID, projectsByOwnerID[0].ID)
		assert.Equal(t, projectByID.OwnerID, &owner.ID)
		assert.Equal(t, projectByID.Name, name)
		assert.Equal(t, projectByID.CompanyName, companyName)
		assert.Equal(t, projectByID.Description, description)
		assert.Equal(t, projectByID.TermsAccepted, 1)
	})

	t.Run("Update project success", func(t *testing.T) {
		oldProjects, err := projects.GetByOwnerID(ctx, owner.ID)

		assert.NoError(t, err)
		assert.NotNil(t, oldProjects)
		assert.Equal(t, len(oldProjects), 1)

		// creating new project with updated values
		newProject := &satellite.Project{
			ID:            oldProjects[0].ID,
			OwnerID:       &owner.ID,
			Description:   newDescription,
			TermsAccepted: 2,
		}

		err = projects.Update(ctx, newProject)

		assert.Nil(t, err)
		assert.NoError(t, err)

		// fetching updated project from db
		newProject, err = projects.Get(ctx, oldProjects[0].ID)

		assert.NoError(t, err)

		assert.Equal(t, newProject.ID, oldProjects[0].ID)
		assert.Equal(t, newProject.OwnerID, &owner.ID)
		assert.Equal(t, newProject.Description, newDescription)
		assert.Equal(t, newProject.TermsAccepted, 2)
	})

	t.Run("Delete project success", func(t *testing.T) {
		oldProjects, err := projects.GetByOwnerID(ctx, owner.ID)

		assert.NoError(t, err)
		assert.NotNil(t, oldProjects)

		err = projects.Delete(ctx, oldProjects[0].ID)

		assert.Nil(t, err)
		assert.NoError(t, err)

		_, err = projects.Get(ctx, oldProjects[0].ID)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("GetAll success", func(t *testing.T) {
		allProjects, err := projects.GetAll(ctx)

		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.Equal(t, len(allProjects), 1)

		newProject := &satellite.Project{
			OwnerID:       &owner.ID,
			Description:   description,
			Name:          name,
			TermsAccepted: 1,
		}

		_, err = projects.Insert(ctx, newProject)

		assert.Nil(t, err)
		assert.NoError(t, err)

		allProjects, err = projects.GetAll(ctx)

		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.Equal(t, len(allProjects), 2)

		newProject2 := &satellite.Project{
			OwnerID:       &owner.ID,
			Description:   description,
			Name:          name,
			TermsAccepted: 1,
		}

		_, err = projects.Insert(ctx, newProject2)

		assert.Nil(t, err)
		assert.NoError(t, err)

		allProjects, err = projects.GetAll(ctx)

		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.Equal(t, len(allProjects), 3)
	})
}

func TestProjectFromDbx(t *testing.T) {
	t.Run("can't create dbo from nil dbx model", func(t *testing.T) {
		project, err := projectFromDBX(nil)

		assert.Nil(t, project)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("can't create dbo from dbx model with invalid ID", func(t *testing.T) {
		dbxProject := dbx.Project{
			Id: []byte("qweqwe"),
		}

		project, err := projectFromDBX(&dbxProject)

		assert.Nil(t, project)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("can't create dbo from dbx model with invalid OwnerID", func(t *testing.T) {
		projectID, err := uuid.New()
		assert.NoError(t, err)
		assert.Nil(t, err)

		dbxProject := dbx.Project{
			Id:      projectID[:],
			OwnerId: []byte("qweqwe"),
		}

		project, err := projectFromDBX(&dbxProject)

		assert.Nil(t, project)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})
}
