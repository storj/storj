// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) { // repositories
		users := db.Console().Users()
		projects := db.Console().Projects()
		var project *console.Project
		var owner *console.User

		rateLimit := 100
		t.Run("Insert project successfully", func(t *testing.T) {
			var err error
			owner, err = users.Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				FullName:     userFullName,
				ShortName:    shortName,
				Email:        email,
				PasswordHash: []byte(pass),
			})
			require.NoError(t, err)
			require.NotNil(t, owner)
			owner, err := users.Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				FullName:     userFullName,
				ShortName:    shortName,
				Email:        email,
				PasswordHash: []byte(pass),
			})
			require.NoError(t, err)
			require.NotNil(t, owner)

			t.Run("Insert project successfully", func(t *testing.T) {
				project = &console.Project{
					Name:        name,
					Description: description,
					OwnerID:     owner.ID,
					RateLimit:   &rateLimit,
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
				require.NotNil(t, project)
				require.NoError(t, err)
			})

			t.Run("Get by projectID success", func(t *testing.T) {
				projectByID, err := projects.Get(ctx, project.ID)
				require.NoError(t, err)
				require.Equal(t, project.ID, projectByID.ID)
				require.Equal(t, name, projectByID.Name)
				require.Equal(t, owner.ID, projectByID.OwnerID)
				require.Equal(t, description, projectByID.Description)
				require.Equal(t, rateLimit, *projectByID.RateLimit)
			})

			t.Run("Update project success", func(t *testing.T) {
				oldProject, err := projects.Get(ctx, project.ID)
				require.NoError(t, err)
				require.NotNil(t, oldProject)

				newRateLimit := 1000

				// creating new project with updated values
				newProject := &console.Project{
					ID:          oldProject.ID,
					Description: newDescription,
					RateLimit:   &newRateLimit,
				}

				err = projects.Update(ctx, newProject)
				require.NoError(t, err)

				// fetching updated project from db
				newProject, err = projects.Get(ctx, oldProject.ID)
				require.NoError(t, err)
				require.Equal(t, oldProject.ID, newProject.ID)
				require.Equal(t, newDescription, newProject.Description)
				require.Equal(t, newRateLimit, *newProject.RateLimit)
			})

			t.Run("Delete project success", func(t *testing.T) {
				oldProject, err := projects.Get(ctx, project.ID)
				require.NoError(t, err)
				require.NotNil(t, oldProject)

				err = projects.Delete(ctx, oldProject.ID)
				require.NoError(t, err)

				_, err = projects.Get(ctx, oldProject.ID)
				require.Error(t, err)
			})

			t.Run("GetAll success", func(t *testing.T) {
				allProjects, err := projects.GetAll(ctx)
				require.NoError(t, err)
				require.Equal(t, 0, len(allProjects))

				newProject := &console.Project{
					Description: description,
					Name:        name,
				}

				_, err = projects.Insert(ctx, newProject)
				require.NoError(t, err)

				allProjects, err = projects.GetAll(ctx)
				require.NoError(t, err)
				require.Equal(t, 1, len(allProjects))

				newProject2 := &console.Project{
					Description: description,
					Name:        name + "2",
				}

				_, err = projects.Insert(ctx, newProject2)
				require.NoError(t, err)

				allProjects, err = projects.GetAll(ctx)
				require.NoError(t, err)
				require.Equal(t, 2, len(allProjects))
			})
		})
	})
}

func TestProjectsList(t *testing.T) {
	const (
		limit  = 5
		length = limit * 4
	)

	rateLimit := 100

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) { // repositories
		// create owner
		owner, err := db.Console().Users().Insert(ctx,
			&console.User{
				ID:           testrand.UUID(),
				FullName:     "Billy H",
				Email:        "billyh@example.com",
				PasswordHash: []byte("example_password"),
				Status:       1,
			},
		)
		require.NoError(t, err)

		projectsDB := db.Console().Projects()

		//create projects
		var projects []console.Project
		for i := 0; i < length; i++ {
			proj, err := projectsDB.Insert(ctx,
				&console.Project{
					Name:        "example",
					Description: "example",
					OwnerID:     owner.ID,
					RateLimit:   &rateLimit,
				},
			)
			require.NoError(t, err)

			projects = append(projects, *proj)
		}

		now := time.Now().Add(time.Second)

		projsPage, err := projectsDB.List(ctx, 0, limit, now)
		require.NoError(t, err)

		projectsList := projsPage.Projects

		for projsPage.Next {
			projsPage, err = projectsDB.List(ctx, projsPage.NextOffset, limit, now)
			require.NoError(t, err)

			projectsList = append(projectsList, projsPage.Projects...)
		}

		require.False(t, projsPage.Next)
		require.Equal(t, int64(0), projsPage.NextOffset)
		require.Equal(t, length, len(projectsList))
		require.Empty(t, cmp.Diff(projects[0], projectsList[0],
			cmp.Transformer("Sort", func(xs []console.Project) []console.Project {
				rs := append([]console.Project{}, xs...)
				sort.Slice(rs, func(i, k int) bool {
					return rs[i].ID.String() < rs[k].ID.String()
				})
				return rs
			})))
	})
}
