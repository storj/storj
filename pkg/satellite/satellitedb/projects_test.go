// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"github.com/skyrings/skyring-common/tools/uuid"
	"storj.io/storj/pkg/satellite/satellitedb/dbx"
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/satellite"
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
		name        = "Storj"
		description = "some description"

		// updated project values
		newName        = "Dropbox"
		newDescription = "some new description"
	)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// creating in-memory db and opening connection
	// to test with real db3 file use this connection string - "../db/accountdb.db3"
	db, err := New("sqlite3", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		assert.NoError(t, err)
	}
	defer ctx.Check(db.Close)

	// creating tables
	err = db.CreateTables()
	if err != nil {
		assert.NoError(t, err)
	}

	// repositories
	users := db.Users()
	projects := db.Projects()

	var user *satellite.User

	t.Run("Can't insert project without user", func(t *testing.T) {

		project := &satellite.Project{
			Name:              name,
			Description:       description,
			IsAgreedWithTerms: false,
		}

		createdCompany, err := projects.Insert(ctx, project)

		assert.Nil(t, createdCompany)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Insert project successfully", func(t *testing.T) {

		user, err = users.Insert(ctx, &satellite.User{
			FirstName:    userName,
			LastName:     lastName,
			Email:        email,
			PasswordHash: []byte(pass),
		})

		assert.NoError(t, err)
		assert.NotNil(t, user)

		project := &satellite.Project{
			UserID: user.ID,

			Name:              name,
			Description:       description,
			IsAgreedWithTerms: false,
		}

		createdProject, err := projects.Insert(ctx, project)

		assert.NotNil(t, createdProject)
		assert.Nil(t, err)
		assert.NoError(t, err)
	})

	t.Run("Get project success", func(t *testing.T) {
		projectByUserID, err := projects.GetByUserID(ctx, user.ID)

		assert.Nil(t, err)
		assert.NoError(t, err)

		assert.Equal(t, projectByUserID.UserID, user.ID)
		assert.Equal(t, projectByUserID.Name, name)
		assert.Equal(t, projectByUserID.Description, description)
		assert.Equal(t, projectByUserID.IsAgreedWithTerms, false)

		projectByID, err := projects.Get(ctx, projectByUserID.ID)

		assert.Nil(t, err)
		assert.NoError(t, err)

		assert.Equal(t, projectByID.ID, projectByUserID.ID)
		assert.Equal(t, projectByID.UserID, user.ID)
		assert.Equal(t, projectByID.Name, name)
		assert.Equal(t, projectByID.Description, description)
		assert.Equal(t, projectByID.IsAgreedWithTerms, false)
	})

	t.Run("Update project success", func(t *testing.T) {
		oldProject, err := projects.GetByUserID(ctx, user.ID)

		assert.NoError(t, err)
		assert.NotNil(t, oldProject)

		// creating new company with updated values
		newProject := &satellite.Project{
			ID:                oldProject.ID,
			UserID:            user.ID,
			Name:              newName,
			Description:       newDescription,
			IsAgreedWithTerms: true,
		}

		err = projects.Update(ctx, newProject)

		assert.Nil(t, err)
		assert.NoError(t, err)

		// fetching updated project from db
		newProject, err = projects.Get(ctx, oldProject.ID)

		assert.NoError(t, err)

		assert.Equal(t, newProject.ID, oldProject.ID)
		assert.Equal(t, newProject.UserID, user.ID)
		assert.Equal(t, newProject.Name, newName)
		assert.Equal(t, newProject.Description, newDescription)
		assert.Equal(t, newProject.IsAgreedWithTerms, true)
	})

	t.Run("Delete project success", func(t *testing.T) {
		oldProject, err := projects.GetByUserID(ctx, user.ID)

		assert.NoError(t, err)
		assert.NotNil(t, oldProject)

		err = projects.Delete(ctx, oldProject.ID)

		assert.Nil(t, err)
		assert.NoError(t, err)

		_, err = projects.Get(ctx, oldProject.ID)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("GetAll success", func(t *testing.T) {
		allProjects, err := projects.GetAll(ctx)

		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.Equal(t, len(allProjects), 0)

		newProject := &satellite.Project{
			UserID:            user.ID,
			Description:       description,
			Name:              name,
			IsAgreedWithTerms: true,
		}

		_, err = projects.Insert(ctx, newProject)

		assert.Nil(t, err)
		assert.NoError(t, err)

		allProjects, err = projects.GetAll(ctx)

		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.Equal(t, len(allProjects), 1)

		newProject2 := &satellite.Project{
			UserID:            user.ID,
			Description:       description,
			Name:              name,
			IsAgreedWithTerms: true,
		}

		_, err = projects.Insert(ctx, newProject2)

		assert.Nil(t, err)
		assert.NoError(t, err)

		allProjects, err = projects.GetAll(ctx)

		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.Equal(t, len(allProjects), 2)
	})
}

func TestProjectFromDbx(t *testing.T) {

	t.Run("can't create dbo from nil dbx model", func(t *testing.T) {
		user, err := projectFromDBX(nil)

		assert.Nil(t, user)
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

	t.Run("can't create dbo from dbx model with invalid UserID", func(t *testing.T) {

		projectID, err := uuid.New()
		assert.NoError(t, err)
		assert.Nil(t, err)

		dbxProject := dbx.Project{
			Id:     projectID[:],
			UserId: []byte("qweqwe"),
		}

		project, err := projectFromDBX(&dbxProject)

		assert.Nil(t, project)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})
}