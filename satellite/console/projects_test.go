// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"context"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProjectsRepository(t *testing.T) {
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
		newName        = "newProjectName"
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
				assert.Equal(t, console.ProjectActive, *project.Status)
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
				require.Equal(t, console.ProjectActive, *project.Status)
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

				// creating new project with updated values.
				newProject := &console.Project{
					ID:          oldProject.ID,
					Name:        newName,
					Description: newDescription,
					RateLimit:   &newRateLimit,
				}

				err = projects.Update(ctx, newProject)
				require.NoError(t, err)

				// fetching updated project from db
				newProject, err = projects.Get(ctx, oldProject.ID)
				require.NoError(t, err)
				require.Equal(t, oldProject.ID, newProject.ID)
				require.Equal(t, newName, newProject.Name)
				require.Equal(t, newDescription, newProject.Description)
				require.Equal(t, newRateLimit, *newProject.RateLimit)
				require.Equal(t, console.ProjectActive, *newProject.Status)

				disabledStatus := console.ProjectDisabled
				newProject.Status = &disabledStatus
				err = projects.Update(ctx, newProject)
				require.NoError(t, err)

				newProject, err = projects.Get(ctx, oldProject.ID)
				require.NoError(t, err)
				require.Equal(t, console.ProjectDisabled, *newProject.Status)
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
				Email:        "billyh@example.test",
				PasswordHash: []byte("example_password"),
				Status:       1,
			},
		)
		require.NoError(t, err)

		projectsDB := db.Console().Projects()

		// Create projects
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
		require.EqualValues(t, 0, projsPage.NextOffset)
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

func TestListPendingDeletionBefore(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		usersRepo := db.Console().Users()
		projectsRepo := db.Console().Projects()

		owner1, err := usersRepo.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "Test User 1",
			Email:        "test1@test.test",
			PasswordHash: testrand.Bytes(8),
		})
		require.NoError(t, err)

		owner2, err := usersRepo.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "Test User 2",
			Email:        "test2@test.test",
			PasswordHash: testrand.Bytes(8),
		})
		require.NoError(t, err)

		now := time.Now().Truncate(time.Minute)
		beforeTime := now.Add(-time.Hour)
		afterTime := now.Add(time.Hour)

		pendingProject1, err := projectsRepo.Insert(ctx, &console.Project{
			ID:      testrand.UUID(),
			Name:    "PendingDeletion1",
			OwnerID: owner1.ID,
		})
		require.NoError(t, err)

		pendingProject2, err := projectsRepo.Insert(ctx, &console.Project{
			ID:      testrand.UUID(),
			Name:    "PendingDeletion2",
			OwnerID: owner2.ID,
		})
		require.NoError(t, err)

		_, err = projectsRepo.Insert(ctx, &console.Project{
			ID:      testrand.UUID(),
			Name:    "Active",
			OwnerID: owner1.ID,
		})
		require.NoError(t, err)

		err = projectsRepo.UpdateStatus(ctx, pendingProject1.ID, console.ProjectPendingDeletion)
		require.NoError(t, err)

		err = projectsRepo.UpdateStatus(ctx, pendingProject2.ID, console.ProjectPendingDeletion)
		require.NoError(t, err)

		page, err := projectsRepo.ListPendingDeletionBefore(ctx, 0, 10, afterTime)
		require.NoError(t, err)
		require.Len(t, page.Ids, 2)
		require.Contains(t, page.Ids, console.ProjectIdOwnerId{
			ProjectID:       pendingProject1.ID,
			ProjectPublicID: pendingProject1.PublicID,
			OwnerID:         owner1.ID,
		})
		require.Contains(t, page.Ids, console.ProjectIdOwnerId{
			ProjectID:       pendingProject2.ID,
			ProjectPublicID: pendingProject2.PublicID,
			OwnerID:         owner2.ID,
		})

		page, err = projectsRepo.ListPendingDeletionBefore(ctx, 0, 1, afterTime)
		require.NoError(t, err)
		require.Len(t, page.Ids, 1)
		require.Equal(t, pendingProject1.ID, page.Ids[0].ProjectID)
		require.Equal(t, pendingProject1.PublicID, page.Ids[0].ProjectPublicID)

		page, err = projectsRepo.ListPendingDeletionBefore(ctx, 1, 1, afterTime)
		require.NoError(t, err)
		require.Len(t, page.Ids, 1)
		require.Equal(t, pendingProject2.ID, page.Ids[0].ProjectID)
		require.Equal(t, pendingProject2.PublicID, page.Ids[0].ProjectPublicID)

		page, err = projectsRepo.ListPendingDeletionBefore(ctx, 0, 10, beforeTime)
		require.NoError(t, err)
		require.Len(t, page.Ids, 0)
	})
}

func TestUpdateStatus(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) { // repositories
		owner, err := db.Console().Users().Insert(ctx,
			&console.User{
				ID:           testrand.UUID(),
				FullName:     "test",
				Email:        "test@example.test",
				PasswordHash: []byte("example_password"),
			},
		)
		require.NoError(t, err)

		projectsDB := db.Console().Projects()

		timestamp := time.Now().Add(10 * time.Hour)
		projectsDB.TestSetNowFn(func() time.Time {
			return timestamp
		})

		proj, err := projectsDB.Insert(ctx,
			&console.Project{
				Name:        "example",
				Description: "example",
				OwnerID:     owner.ID,
			},
		)
		require.NoError(t, err)
		require.Equal(t, console.ProjectActive, *proj.Status)

		err = projectsDB.UpdateStatus(ctx, proj.ID, console.ProjectDisabled)
		require.NoError(t, err)

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		require.Equal(t, console.ProjectDisabled, *proj.Status)
		require.NotNil(t, proj.StatusUpdatedAt)
		assert.WithinDuration(t, timestamp, *proj.StatusUpdatedAt, time.Minute)

		// get db again to be sure got the same instance
		projectsDB = db.Console().Projects()

		active := console.ProjectActive
		proj.Status = &active
		err = projectsDB.Update(ctx, proj)
		require.NoError(t, err)

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		require.Equal(t, console.ProjectActive, *proj.Status)
		require.NotNil(t, proj.StatusUpdatedAt)
		require.WithinDuration(t, timestamp, *proj.StatusUpdatedAt, time.Minute)

		// test that status_updated_at is set correctly when updating status in transaction
		newTimestamp := timestamp.Add(24 * time.Hour)
		projectsDB.TestSetNowFn(func() time.Time { return newTimestamp })
		err = db.Console().WithTx(ctx, func(ctx context.Context, tx console.DBTx) error {
			err := tx.Projects().UpdateStatus(ctx, proj.ID, console.ProjectPendingDeletion)
			if err != nil {
				return err
			}

			proj, err = tx.Projects().Get(ctx, proj.ID)
			if err != nil {
				return err
			}

			return nil
		})
		require.NoError(t, err)
		require.Equal(t, console.ProjectPendingDeletion, *proj.Status)
		require.NotNil(t, proj.StatusUpdatedAt)
		require.WithinDuration(t, newTimestamp, *proj.StatusUpdatedAt, time.Minute)
	})
}

func TestGetOwnActive(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) { // repositories
		owner, err := db.Console().Users().Insert(ctx,
			&console.User{
				ID:           testrand.UUID(),
				FullName:     "test",
				Email:        "test@example.test",
				PasswordHash: []byte("example_password"),
			},
		)
		require.NoError(t, err)

		projectsDB := db.Console().Projects()

		proj1, err := projectsDB.Insert(ctx, &console.Project{Name: "example1", OwnerID: owner.ID})
		require.NoError(t, err)
		require.Equal(t, console.ProjectActive, *proj1.Status)

		proj2, err := projectsDB.Insert(ctx, &console.Project{Name: "example2", OwnerID: owner.ID})
		require.NoError(t, err)
		require.Equal(t, console.ProjectActive, *proj2.Status)

		projects, err := projectsDB.GetOwnActive(ctx, owner.ID)
		require.NoError(t, err)
		require.Len(t, projects, 2)

		err = projectsDB.UpdateStatus(ctx, proj1.ID, console.ProjectDisabled)
		require.NoError(t, err)

		projects, err = projectsDB.GetOwnActive(ctx, owner.ID)
		require.NoError(t, err)
		require.Len(t, projects, 1)
		require.Equal(t, proj2.ID, projects[0].ID)
	})
}

func TestProjectsListByOwner(t *testing.T) {
	const (
		limit      = 5
		length     = limit*4 - 1 // make length offset from page size so we can test incomplete page at end
		totalPages = 4
	)

	rateLimit := 100

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		owner1, err := db.Console().Users().Insert(ctx,
			&console.User{
				ID:           testrand.UUID(),
				FullName:     "Billy H",
				Email:        "billyh@example.test",
				PasswordHash: []byte("example_password"),
				Status:       1,
			},
		)
		require.NoError(t, err)

		owner2, err := db.Console().Users().Insert(ctx,
			&console.User{
				ID:           testrand.UUID(),
				FullName:     "James H",
				Email:        "james@example.test",
				PasswordHash: []byte("example_password_2"),
				Status:       1,
			},
		)
		require.NoError(t, err)

		projectsDB := db.Console().Projects()
		projectMembersDB := db.Console().ProjectMembers()

		// Create projects
		var owner1Projects []console.Project
		var owner2Projects []console.Project
		for i := 0; i < length; i++ {
			proj1, err := projectsDB.Insert(ctx,
				&console.Project{
					Name:        "owner1example" + strconv.Itoa(i),
					Description: "example",
					OwnerID:     owner1.ID,
					RateLimit:   &rateLimit,
				},
			)
			require.NoError(t, err)

			proj2, err := projectsDB.Insert(ctx,
				&console.Project{
					Name:        "owner2example" + strconv.Itoa(i),
					Description: "example",
					OwnerID:     owner2.ID,
					RateLimit:   &rateLimit,
				},
			)
			require.NoError(t, err)

			// insert 0, 1, or 2 project members
			numMembers := i % 3
			switch numMembers {
			case 1:
				_, err = projectMembersDB.Insert(ctx, owner1.ID, proj1.ID, console.RoleAdmin)
				require.NoError(t, err)
				_, err = projectMembersDB.Insert(ctx, owner2.ID, proj2.ID, console.RoleAdmin)
				require.NoError(t, err)
			case 2:
				_, err = projectMembersDB.Insert(ctx, owner1.ID, proj1.ID, console.RoleAdmin)
				require.NoError(t, err)
				_, err = projectMembersDB.Insert(ctx, owner2.ID, proj1.ID, console.RoleAdmin)
				require.NoError(t, err)
				_, err = projectMembersDB.Insert(ctx, owner1.ID, proj2.ID, console.RoleAdmin)
				require.NoError(t, err)
				_, err = projectMembersDB.Insert(ctx, owner2.ID, proj2.ID, console.RoleAdmin)
				require.NoError(t, err)
			}
			proj1.MemberCount = numMembers
			proj2.MemberCount = numMembers

			owner1Projects = append(owner1Projects, *proj1)
			owner2Projects = append(owner2Projects, *proj2)
		}

		// test listing for each
		testCases := []struct {
			id               uuid.UUID
			originalProjects []console.Project
		}{
			{id: owner1.ID, originalProjects: owner1Projects},
			{id: owner2.ID, originalProjects: owner2Projects},
		}
		for _, tt := range testCases {
			cursor := &console.ProjectsCursor{
				Limit: limit,
				Page:  1,
			}
			projsPage, err := projectsDB.ListByOwnerID(ctx, tt.id, *cursor)
			require.NoError(t, err)
			require.Len(t, projsPage.Projects, limit)
			require.EqualValues(t, 1, projsPage.CurrentPage)
			require.EqualValues(t, totalPages, projsPage.PageCount)
			require.EqualValues(t, length, projsPage.TotalCount)

			ownerProjectsDB := projsPage.Projects

			for projsPage.Next {
				cursor.Page++
				projsPage, err = projectsDB.ListByOwnerID(ctx, tt.id, *cursor)
				require.NoError(t, err)
				// number of projects should not exceed page limit
				require.True(t, len(projsPage.Projects) > 0 && len(projsPage.Projects) <= limit)
				require.EqualValues(t, cursor.Page, projsPage.CurrentPage)
				require.EqualValues(t, totalPages, projsPage.PageCount)
				require.EqualValues(t, length, projsPage.TotalCount)

				ownerProjectsDB = append(ownerProjectsDB, projsPage.Projects...)
			}

			require.False(t, projsPage.Next)
			require.EqualValues(t, 0, projsPage.NextOffset)
			require.Equal(t, length, len(ownerProjectsDB))
			// sort originalProjects by Name in alphabetical order
			originalProjects := tt.originalProjects
			sort.SliceStable(originalProjects, func(i, j int) bool {
				return strings.Compare(originalProjects[i].Name, originalProjects[j].Name) < 0
			})
			for i, p := range ownerProjectsDB {
				// expect response projects to be in alphabetical order
				require.Equal(t, originalProjects[i].Name, p.Name)
				require.Equal(t, originalProjects[i].MemberCount, p.MemberCount)
			}

			err = projectsDB.UpdateStatus(ctx, ownerProjectsDB[0].ID, console.ProjectDisabled)
			require.NoError(t, err)

			projsPage, err = projectsDB.ListByOwnerID(ctx, tt.id, *cursor)
			require.NoError(t, err)
			require.EqualValues(t, length, projsPage.TotalCount)

			projsPage, err = projectsDB.ListActiveByOwnerID(ctx, tt.id, *cursor)
			require.NoError(t, err)
			require.EqualValues(t, length-1, projsPage.TotalCount)
		}
	})
}

func TestGetMaxBuckets(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		maxCount := 100
		consoleDB := db.Console()
		project, err := consoleDB.Projects().Insert(ctx, &console.Project{Name: "testproject1", MaxBuckets: &maxCount})
		require.NoError(t, err)
		projectsDB := db.Console().Projects()
		max, err := projectsDB.GetMaxBuckets(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, maxCount, *max)
	})
}

func TestValidateNameAndDescription(t *testing.T) {
	t.Run("Project name and description validation test", func(t *testing.T) {
		validDescription := randString(100)

		// update project with empty name.
		err := console.ValidateNameAndDescription("", validDescription)
		require.Error(t, err)

		notValidName := randString(21)

		// update project with too long name.
		err = console.ValidateNameAndDescription(notValidName, validDescription)
		require.Error(t, err)

		validName := randString(15)
		notValidDescription := randString(101)

		// update project with too long description.
		err = console.ValidateNameAndDescription(validName, notValidDescription)
		require.Error(t, err)

		// update project with valid name and description.
		err = console.ValidateNameAndDescription(validName, validDescription)
		require.NoError(t, err)
	})
}

func TestRateLimit_ProjectRateLimitZero(t *testing.T) {
	rateLimit := 2
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Rate = float64(rateLimit)
				// Make limit cache to refresh as quickly as possible
				// if it starts to become flaky, we can then add sleeps between
				// the cache update and the API calls
				config.Metainfo.RateLimiter.CacheExpiration = time.Millisecond
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, projects, 1)

		zeroRateLimit := 0
		err = satellite.DB.Console().Projects().UpdateRateLimit(ctx, projects[0].ID, &zeroRateLimit)
		require.NoError(t, err)

		var group errs2.Group
		for i := 0; i <= rateLimit; i++ {
			group.Go(func() error {
				_, err := ul.ListBuckets(ctx, satellite)
				return err
			})
		}
		groupErrs := group.Wait()
		require.Len(t, groupErrs, 3)
	})
}

func TestBurstLimit_ProjectBurstLimitZero(t *testing.T) {
	rateLimit := 2
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Metainfo.RateLimiter.Rate = float64(rateLimit)
				// Make limit cache to refresh as quickly as possible
				// if it starts to become flaky, we can then add sleeps between
				// the cache update and the API calls
				config.Metainfo.RateLimiter.CacheExpiration = time.Millisecond
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		projects, err := satellite.DB.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, projects, 1)

		zeroRateLimit := 0
		err = satellite.DB.Console().Projects().UpdateBurstLimit(ctx, projects[0].ID, &zeroRateLimit)
		require.NoError(t, err)

		var group errs2.Group
		for i := 0; i <= rateLimit; i++ {
			group.Go(func() error {
				_, err := ul.ListBuckets(ctx, satellite)
				return err
			})
		}
		groupErrs := group.Wait()
		require.Len(t, groupErrs, 3)
	})
}

func randString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
