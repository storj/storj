// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProjectsRepository(t *testing.T) {
	//testing constants
	const (
		// for user
		shortName    = "lastName"
		email        = "email@mail.test"
		pass         = "123456"
		userFullName = "name"

		// for project
		name        = "Project"
		description = "some description"

		// updated project values
		newDescription = "some new description"
	)

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		// repositories
		users := db.Console().Users()
		projects := db.Console().Projects()
		var project *console.Project
		var owner *console.User

		t.Run("Insert project successfully", func(t *testing.T) {
			var err error
			owner, err = users.Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				FullName:     userFullName,
				ShortName:    shortName,
				Email:        email,
				PasswordHash: []byte(pass),
			})
			assert.NoError(t, err)
			assert.NotNil(t, owner)

			project = &console.Project{
				Name:        name,
				Description: description,
				OwnerID:     owner.ID,
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
			assert.Equal(t, projectByID.OwnerID, owner.ID)
			assert.Equal(t, projectByID.Description, description)
		})

		t.Run("Get by projectID success", func(t *testing.T) {
			projectByID, err := projects.Get(ctx, project.ID)
			assert.NoError(t, err)
			assert.Equal(t, projectByID.ID, project.ID)
			assert.Equal(t, projectByID.Name, name)
			assert.Equal(t, projectByID.OwnerID, owner.ID)
			assert.Equal(t, projectByID.Description, description)
		})

		t.Run("Update project success", func(t *testing.T) {
			oldProject, err := projects.Get(ctx, project.ID)
			assert.NoError(t, err)
			assert.NotNil(t, oldProject)

			// creating new project with updated values
			newProject := &console.Project{
				ID:          oldProject.ID,
				Description: newDescription,
			}

			err = projects.Update(ctx, newProject)
			assert.NoError(t, err)

			// fetching updated project from db
			newProject, err = projects.Get(ctx, oldProject.ID)
			assert.NoError(t, err)
			assert.Equal(t, newProject.ID, oldProject.ID)
			assert.Equal(t, newProject.Description, newDescription)
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
				Description: description,
				Name:        name,
			}

			_, err = projects.Insert(ctx, newProject)
			assert.NoError(t, err)

			allProjects, err = projects.GetAll(ctx)
			assert.NoError(t, err)
			assert.Equal(t, len(allProjects), 1)

			newProject2 := &console.Project{
				Description: description,
				Name:        name + "2",
			}

			_, err = projects.Insert(ctx, newProject2)
			assert.NoError(t, err)

			allProjects, err = projects.GetAll(ctx)
			assert.NoError(t, err)
			assert.Equal(t, len(allProjects), 2)
		})
	})
}
