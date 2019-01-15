// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"testing"

	"storj.io/storj/satellite/console"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	dbx "storj.io/storj/satellite/satellitedb/consoledbx"
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
		description = "some description"

		// updated project values
		newDescription = "some new description"
	)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// creating in-memory db and opening connection
	// to test with real db3 file use this connection string - "../db/accountdb.db3"
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
	var project *console.Project
	var owner *console.User

	t.Run("Insert project successfully", func(t *testing.T) {
		owner, err = users.Insert(ctx, &console.User{
			FirstName:    userName,
			LastName:     lastName,
			Email:        email,
			PasswordHash: []byte(pass),
		})
		assert.NoError(t, err)
		assert.NotNil(t, owner)

		project = &console.Project{
			Name:          name,
			Description:   description,
			TermsAccepted: 1,
		}

		project, err = projects.Insert(ctx, project)
		assert.NotNil(t, project)
		assert.NoError(t, err)
	})

	t.Run("Get project success", func(t *testing.T) {
		projectByID, err := projects.Get(ctx, project.ID)
		assert.NoError(t, err)
		assert.Equal(t, projectByID.ID, project.ID)
		assert.Equal(t, projectByID.Name, name)
		assert.Equal(t, projectByID.Description, description)
		assert.Equal(t, projectByID.TermsAccepted, 1)
	})

	t.Run("Get by projectID success", func(t *testing.T) {
		projectByID, err := projects.Get(ctx, project.ID)
		assert.NoError(t, err)
		assert.Equal(t, projectByID.ID, project.ID)
		assert.Equal(t, projectByID.Name, name)
		assert.Equal(t, projectByID.Description, description)
		assert.Equal(t, projectByID.TermsAccepted, 1)
	})

	t.Run("Update project success", func(t *testing.T) {
		oldProject, err := projects.Get(ctx, project.ID)
		assert.NoError(t, err)
		assert.NotNil(t, oldProject)

		// creating new project with updated values
		newProject := &console.Project{
			ID:            oldProject.ID,
			Description:   newDescription,
			TermsAccepted: 2,
		}

		err = projects.Update(ctx, newProject)
		assert.NoError(t, err)

		// fetching updated project from db
		newProject, err = projects.Get(ctx, oldProject.ID)
		assert.NoError(t, err)
		assert.Equal(t, newProject.ID, oldProject.ID)
		assert.Equal(t, newProject.Description, newDescription)
		assert.Equal(t, newProject.TermsAccepted, 2)
	})

	t.Run("Delete project success", func(t *testing.T) {
		oldProject, err := projects.Get(ctx, project.ID)
		assert.NoError(t, err)
		assert.NotNil(t, oldProject)

		err = projects.Delete(ctx, oldProject.ID)
		assert.NoError(t, err)

		_, err = projects.Get(ctx, oldProject.ID)
		assert.Error(t, err)
	})

	t.Run("GetAll success", func(t *testing.T) {
		allProjects, err := projects.GetAll(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(allProjects), 0)

		newProject := &console.Project{
			Description:   description,
			Name:          name,
			TermsAccepted: 1,
		}

		_, err = projects.Insert(ctx, newProject)
		assert.NoError(t, err)

		allProjects, err = projects.GetAll(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(allProjects), 1)

		newProject2 := &console.Project{
			Description:   description,
			Name:          name,
			TermsAccepted: 1,
		}

		_, err = projects.Insert(ctx, newProject2)
		assert.NoError(t, err)

		allProjects, err = projects.GetAll(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(allProjects), 2)
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
}
