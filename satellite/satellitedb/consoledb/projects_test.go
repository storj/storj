// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProjectsGetByPublicID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projects := db.Console().Projects()

		prj, err := projects.Insert(ctx, &console.Project{
			Name:        "ProjectName",
			Description: "projects description",
		})
		require.NoError(t, err)
		require.NotNil(t, prj)

		pubID := prj.PublicID
		require.NotNil(t, pubID)
		require.False(t, pubID.IsZero())

		prj, err = projects.GetByPublicID(ctx, pubID)
		require.NoError(t, err)
		require.Equal(t, pubID, prj.PublicID)
	})
}

func TestProjectsGetPublicID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projects := db.Console().Projects()

		prj, err := projects.Insert(ctx, &console.Project{
			Name:        "ProjectName",
			Description: "projects description",
		})
		require.NoError(t, err)
		require.NotNil(t, prj)

		publicID, err := projects.GetPublicID(ctx, prj.ID)
		require.NoError(t, err)
		require.Equal(t, prj.PublicID, publicID)
	})
}

func TestProjectsGetSalt(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projects := db.Console().Projects()

		prj, err := projects.Insert(ctx, &console.Project{
			Name:        "ProjectName",
			Description: "projects description",
		})
		require.NoError(t, err)
		require.NotNil(t, prj)

		salt, err := projects.GetSalt(ctx, prj.ID)
		require.NoError(t, err)

		_, err = uuid.FromBytes(salt)
		require.NoError(t, err)
	})
}

func TestUpdateProjectUsageLimits(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		limits := console.UsageLimits{Storage: rand.Int63(), Bandwidth: rand.Int63(), Segment: rand.Int63()}
		projectsRepo := db.Console().Projects()

		proj, err := projectsRepo.Insert(ctx, &console.Project{})
		require.NoError(t, err)
		require.NotNil(t, proj)

		err = projectsRepo.UpdateUsageLimits(ctx, proj.ID, limits)
		require.NoError(t, err)

		proj, err = projectsRepo.Get(ctx, proj.ID)
		require.NoError(t, err)
		require.Equal(t, limits.Bandwidth, proj.BandwidthLimit.Int64())
		require.Equal(t, limits.Storage, proj.StorageLimit.Int64())
		require.Equal(t, limits.Segment, *proj.SegmentLimit)
	})
}

func TestGetProjectsByUserID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projectsRepo := db.Console().Projects()
		users := db.Console().Users()
		projectMembers := db.Console().ProjectMembers()

		user1, err := users.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "user1@mail.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		user2, err := users.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "user2@mail.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		passphraseEnc := testrand.Bytes(2 * memory.B)
		proj, err := projectsRepo.Insert(ctx, &console.Project{
			Name:          "Project",
			OwnerID:       user1.ID,
			PassphraseEnc: passphraseEnc,
		})
		require.NoError(t, err)

		_, err = projectMembers.Insert(ctx, user1.ID, proj.ID, console.RoleAdmin)
		require.NoError(t, err)

		projects, err := projectsRepo.GetByUserID(ctx, user1.ID)
		require.NoError(t, err)
		require.Len(t, projects, 1)
		require.Equal(t, 1, projects[0].MemberCount)
		require.NotNil(t, projects[0].PassphraseEnc)
		require.EqualValues(t, passphraseEnc, projects[0].PassphraseEnc)

		_, err = projectMembers.Insert(ctx, user2.ID, proj.ID, console.RoleAdmin)
		require.NoError(t, err)

		projects, err = projectsRepo.GetByUserID(ctx, user1.ID)
		require.NoError(t, err)
		require.Len(t, projects, 1)
		require.Equal(t, 2, projects[0].MemberCount)

		projects, err = projectsRepo.GetActiveByUserID(ctx, user1.ID)
		require.NoError(t, err)
		require.Len(t, projects, 1)

		for _, status := range []console.ProjectStatus{console.ProjectDisabled, console.ProjectPendingDeletion} {
			err = projectsRepo.UpdateStatus(ctx, proj.ID, status)
			require.NoError(t, err)

			projects, err = projectsRepo.GetByUserID(ctx, user1.ID)
			require.NoError(t, err)
			require.Len(t, projects, 1)

			projects, err = projectsRepo.GetActiveByUserID(ctx, user1.ID)
			require.NoError(t, err)
			require.Len(t, projects, 0)

			err = projectsRepo.UpdateStatus(ctx, proj.ID, console.ProjectActive)
			require.NoError(t, err)
		}
	})
}

func TestUpdateAllProjectLimits(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projects := db.Console().Projects()
		users := db.Console().Users()

		user, err := users.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "user@mail.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		proj, err := projects.Insert(ctx, &console.Project{
			Name:    "Project",
			OwnerID: user.ID,
		})
		require.NoError(t, err)

		defaultStorage := proj.StorageLimit
		defaultBandwidth := proj.BandwidthLimit
		defaultSegment := proj.SegmentLimit
		defaultBuckets := proj.MaxBuckets
		defaultRate := proj.RateLimit
		defaultBurst := proj.BurstLimit

		require.Nil(t, defaultStorage)
		require.Nil(t, defaultBandwidth)
		require.Nil(t, defaultBuckets)
		require.Nil(t, defaultRate)
		require.Nil(t, defaultBurst)
		// segment_limit column default is set to 1000000 (satellite/satellitedb/dbx/project.dbx)
		require.NotNil(t, defaultSegment)
		require.Equal(t, int64(1000000), *defaultSegment)

		newStorage := int64(2000000)
		newBandwidth := int64(1000000)
		newSegment := *defaultSegment * 2
		newBuckets := 100
		newRate := 1000
		newBurst := 2000

		require.NoError(t, projects.UpdateAllLimits(ctx, proj.ID, &newStorage, &newBandwidth, &newSegment, &newBuckets, &newRate, &newBurst))

		p, err := projects.Get(ctx, proj.ID)
		require.NoError(t, err)

		require.NotNil(t, p.StorageLimit)
		require.Equal(t, newStorage, p.StorageLimit.Int64())
		require.NotNil(t, p.BandwidthLimit)
		require.Equal(t, newBandwidth, p.BandwidthLimit.Int64())
		require.NotNil(t, p.SegmentLimit)
		require.Equal(t, newSegment, *p.SegmentLimit)
		require.NotNil(t, p.MaxBuckets)
		require.Equal(t, newBuckets, *p.MaxBuckets)
		require.NotNil(t, p.RateLimit)
		require.Equal(t, newRate, *p.RateLimit)
		require.NotNil(t, p.BurstLimit)
		require.Equal(t, newBurst, *p.BurstLimit)

		require.NoError(t, projects.UpdateAllLimits(ctx, proj.ID, nil, nil, nil, nil, nil, nil))

		p, err = projects.Get(ctx, proj.ID)
		require.NoError(t, err)

		require.Nil(t, p.StorageLimit)
		require.Nil(t, p.BandwidthLimit)
		require.Nil(t, p.SegmentLimit)
		require.Nil(t, p.MaxBuckets)
		require.Nil(t, p.RateLimit)
		require.Nil(t, p.BurstLimit)
	})
}

func TestProjectUpdateNotificationFlags(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projectsRepo := db.Console().Projects()

		proj, err := projectsRepo.Insert(ctx, &console.Project{Name: "test"})
		require.NoError(t, err)

		t.Run("nil on new project", func(t *testing.T) {
			p, err := projectsRepo.Get(ctx, proj.ID)
			require.NoError(t, err)
			require.Nil(t, p.NotificationFlags)
		})

		t.Run("persisted after update", func(t *testing.T) {
			flags := 3 // StorageNotificationsEnabled | StorageUsage80
			proj.NotificationFlags = &flags
			require.NoError(t, projectsRepo.Update(ctx, proj))

			p, err := projectsRepo.Get(ctx, proj.ID)
			require.NoError(t, err)
			require.NotNil(t, p.NotificationFlags)
			require.Equal(t, flags, *p.NotificationFlags)
		})

		t.Run("not cleared when nil in update", func(t *testing.T) {
			proj.NotificationFlags = nil
			require.NoError(t, projectsRepo.Update(ctx, proj))

			p, err := projectsRepo.Get(ctx, proj.ID)
			require.NoError(t, err)
			// flags should still be set since nil means "don't update"
			require.NotNil(t, p.NotificationFlags)
		})
	})
}

func TestUpdateLimitsGeneric(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projects := db.Console().Projects()
		users := db.Console().Users()

		user, err := users.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "user@mail.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		proj, err := projects.Insert(ctx, &console.Project{
			Name:    "Project",
			OwnerID: user.ID,
		})
		require.NoError(t, err)

		equalValues := func(a, b *int64) bool {
			if a == nil && b == nil {
				return true
			}
			if (a == nil && b != nil) || (a != nil && b == nil) {
				return false
			}
			return *a == *b
		}
		equalValuesMemory := func(a *memory.Size, b *int64) bool {
			var val *int64
			if a != nil {
				newVal := a.Int64()
				val = &newVal
			}
			return equalValues(val, b)

		}
		equalValuesInt := func(a *int, b *int64) bool {
			var val *int64
			if a != nil {
				newVal := int64(*a)
				val = &newVal
			}
			return equalValues(val, b)

		}
		equalLimits := func(p *console.Project, kind console.LimitKind, expected *int64) bool {
			switch kind {
			case console.StorageLimit:
				return equalValuesMemory(p.StorageLimit, expected)
			case console.BandwidthLimit:
				return equalValuesMemory(p.BandwidthLimit, expected)
			case console.UserSetStorageLimit:
				return equalValuesMemory(p.UserSpecifiedStorageLimit, expected)
			case console.UserSetBandwidthLimit:
				return equalValuesMemory(p.UserSpecifiedBandwidthLimit, expected)
			case console.SegmentLimit:
				return equalValues(p.SegmentLimit, expected)
			case console.BucketsLimit:
				return equalValuesInt(p.MaxBuckets, expected)
			case console.RateLimit:
				return equalValuesInt(p.RateLimit, expected)
			case console.BurstLimit:
				return equalValuesInt(p.BurstLimit, expected)
			case console.RateLimitHead:
				return equalValuesInt(p.RateLimitHead, expected)
			case console.BurstLimitHead:
				return equalValuesInt(p.BurstLimitHead, expected)
			case console.RateLimitGet:
				return equalValuesInt(p.RateLimitGet, expected)
			case console.BurstLimitGet:
				return equalValuesInt(p.BurstLimitGet, expected)
			case console.RateLimitPut:
				return equalValuesInt(p.RateLimitPut, expected)
			case console.BurstLimitPut:
				return equalValuesInt(p.BurstLimitPut, expected)
			case console.RateLimitList:
				return equalValuesInt(p.RateLimitList, expected)
			case console.BurstLimitList:
				return equalValuesInt(p.BurstLimitList, expected)
			case console.RateLimitDelete:
				return equalValuesInt(p.RateLimitDelete, expected)
			case console.BurstLimitDelete:
				return equalValuesInt(p.BurstLimitDelete, expected)
			default:
				return false
			}
		}

		allLimitKinds := []console.LimitKind{
			console.StorageLimit,
			console.BandwidthLimit,
			console.UserSetStorageLimit,
			console.UserSetBandwidthLimit,
			console.SegmentLimit,
			console.BucketsLimit,
			console.RateLimit,
			console.BurstLimit,
			console.RateLimitHead,
			console.BurstLimitHead,
			console.RateLimitGet,
			console.BurstLimitGet,
			console.RateLimitPut,
			console.BurstLimitPut,
			console.RateLimitList,
			console.BurstLimitList,
			console.RateLimitDelete,
			console.BurstLimitDelete,
		}
		// test updating all limits to different values, individually
		for i, kind := range allLimitKinds {
			value := int64(i + 1)
			err := projects.UpdateLimitsGeneric(ctx, proj.ID, []console.Limit{
				{Kind: kind, Value: &value},
			})
			require.NoError(t, err)
		}
		proj, err = projects.Get(ctx, proj.ID)
		require.NoError(t, err)
		for i, kind := range allLimitKinds {
			value := int64(i + 1)
			require.True(t, equalLimits(proj, kind, &value), fmt.Sprintf("limit kind %d", kind))
		}

		// test updating all limits to different values, at the same time
		toUpdate := []console.Limit{}
		for i, kind := range allLimitKinds {
			value := int64(100 * (i + 1))
			toUpdate = append(toUpdate, console.Limit{
				Kind:  kind,
				Value: &value,
			})
		}
		err = projects.UpdateLimitsGeneric(ctx, proj.ID, toUpdate)
		require.NoError(t, err)

		proj, err = projects.Get(ctx, proj.ID)
		require.NoError(t, err)
		for i, kind := range allLimitKinds {
			value := int64(100 * (i + 1))
			require.True(t, equalLimits(proj, kind, &value), fmt.Sprintf("limit kind %d", kind))
		}

		// test updating all limits to nil
		toUpdate = []console.Limit{}
		for _, kind := range allLimitKinds {
			toUpdate = append(toUpdate, console.Limit{
				Kind:  kind,
				Value: nil,
			})
		}
		err = projects.UpdateLimitsGeneric(ctx, proj.ID, toUpdate)
		require.NoError(t, err)

		proj, err = projects.Get(ctx, proj.ID)
		require.NoError(t, err)
		for _, kind := range allLimitKinds {
			require.True(t, equalLimits(proj, kind, nil), fmt.Sprintf("limit kind %d", kind))
		}

		// test updating invalid limit type
		value := int64(5000)
		err = projects.UpdateLimitsGeneric(ctx, proj.ID, []console.Limit{
			{Kind: console.SegmentLimit, Value: &value},
			{Kind: console.LimitKind(-1), Value: &value}, // invalid kind
		})
		require.Error(t, err)
		proj, err = projects.Get(ctx, proj.ID)
		require.NoError(t, err)
		// valid limitkind should not have been updated
		require.Nil(t, proj.SegmentLimit)
	})
}

func TestListActiveWithOwners(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		usersDB := db.Console().Users()
		projectsDB := db.Console().Projects()

		tenantID := "test-tenant"
		insertUser := func(email string, status console.UserStatus, tenantID *string) *console.User {
			user, err := usersDB.Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				FullName:     email,
				Email:        email,
				TenantID:     tenantID,
				PasswordHash: []byte("testpassword"),
			})
			require.NoError(t, err)
			require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{Status: &status}))
			return user
		}
		insertProject := func(owner *console.User, name string, status console.ProjectStatus) *console.Project {
			project, err := projectsDB.Insert(ctx, &console.Project{
				ID:       testrand.UUID(),
				PublicID: testrand.UUID(),
				Name:     name,
				OwnerID:  owner.ID,
			})
			require.NoError(t, err)
			require.NoError(t, projectsDB.UpdateStatus(ctx, project.ID, status))
			return project
		}

		userB := insertUser("b@mail.test", console.Active, &tenantID)
		userA := insertUser("a@mail.test", console.Active, &tenantID)
		userC := insertUser("c@mail.test", console.Active, nil)
		inactiveUser := insertUser("d@mail.test", console.Inactive, &tenantID)

		projectB1 := insertProject(userB, "b1", console.ProjectActive)
		projectA2 := insertProject(userA, "a2", console.ProjectActive)
		projectA1 := insertProject(userA, "a1", console.ProjectActive)
		projectC1 := insertProject(userC, "c1", console.ProjectActive)
		insertProject(userA, "disabled", console.ProjectDisabled)
		insertProject(inactiveUser, "d1", console.ProjectActive)

		ids := func(projects ...*console.Project) []uuid.UUID {
			ids := make([]uuid.UUID, 0, len(projects))
			for _, p := range projects {
				ids = append(ids, p.ID)
			}
			return ids
		}
		list := func(params console.ListProjectsWithOwnersParams) []uuid.UUID {
			if params.Limit == 0 {
				params.Limit = 10
			}
			page, err := projectsDB.ListActiveWithOwners(ctx, params)
			require.NoError(t, err)
			require.Nil(t, page.Next)
			ids := make([]uuid.UUID, 0, len(page.Projects))
			for _, p := range page.Projects {
				ids = append(ids, p.ID)
			}
			return ids
		}

		t.Run("fields", func(t *testing.T) {
			page, err := projectsDB.ListActiveWithOwners(ctx, console.ListProjectsWithOwnersParams{Limit: 1})
			require.NoError(t, err)
			require.Len(t, page.Projects, 1)

			got := page.Projects[0]
			require.Equal(t, projectA1.ID, got.ID)
			require.Equal(t, projectA1.PublicID, got.PublicID)
			require.Equal(t, projectA1.Name, got.Name)
			require.Equal(t, userA.ID, got.OwnerID)
			require.Equal(t, userA.Email, got.OwnerEmail)
			require.WithinDuration(t, projectA1.CreatedAt, got.CreatedAt, time.Second)
		})

		t.Run("filters", func(t *testing.T) {
			// disabled projects and projects of inactive users are left out.
			require.Equal(t, ids(projectA1, projectA2, projectB1, projectC1), list(console.ListProjectsWithOwnersParams{}))
			require.Equal(t, ids(projectA1, projectA2, projectB1), list(console.ListProjectsWithOwnersParams{TenantID: &tenantID}))

			otherTenant := "other-tenant"
			require.Empty(t, list(console.ListProjectsWithOwnersParams{TenantID: &otherTenant}))
		})

		t.Run("usage", func(t *testing.T) {
			since := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
			before := since.AddDate(0, 1, 0)
			withUsage := console.ListProjectsWithOwnersParams{TenantID: &tenantID, UsageSince: since, UsageBefore: before}

			require.Empty(t, list(withUsage))

			// usage outside of the period doesn't count.
			require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName: "bucket", ProjectID: projectA1.ID, IntervalStart: before, TotalBytes: 1,
			}))
			require.Empty(t, list(withUsage))

			require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName: "bucket", ProjectID: projectA1.ID, IntervalStart: since.Add(time.Hour), TotalBytes: 1,
			}))
			require.NoError(t, db.Orders().UpdateBucketBandwidthSettle(ctx, projectA2.ID, []byte("bucket"), pb.PieceAction_GET, 1, 0, since.AddDate(0, 0, 3)))
			require.NoError(t, db.RetentionRemainderCharges().Upsert(ctx, accounting.RetentionRemainderCharge{
				ProjectID: projectB1.ID, BucketName: "bucket", DeletedAt: since, RemainderByteHours: 1, ProductID: 1,
			}))
			// usage of a project of another tenant.
			require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName: "bucket", ProjectID: projectC1.ID, IntervalStart: since.Add(time.Hour), TotalBytes: 1,
			}))

			require.Equal(t, ids(projectA1, projectA2, projectB1), list(withUsage))
		})

		t.Run("paging", func(t *testing.T) {
			pageIDs := func(page *console.ProjectsWithOwnersPage) []uuid.UUID {
				ids := make([]uuid.UUID, 0, len(page.Projects))
				for _, p := range page.Projects {
					ids = append(ids, p.ID)
				}
				return ids
			}

			page, err := projectsDB.ListActiveWithOwners(ctx, console.ListProjectsWithOwnersParams{Limit: 3})
			require.NoError(t, err)
			require.Equal(t, ids(projectA1, projectA2, projectB1), pageIDs(page))
			require.Equal(t, &console.ProjectWithOwnerCursor{
				OwnerNormalizedEmail: strings.ToUpper(userB.Email),
				Name:                 projectB1.Name,
				PublicID:             projectB1.PublicID,
			}, page.Next)
			afterB1 := *page.Next

			page, err = projectsDB.ListActiveWithOwners(ctx, console.ListProjectsWithOwnersParams{Limit: 3, After: afterB1})
			require.NoError(t, err)
			require.Equal(t, ids(projectC1), pageIDs(page))
			require.Nil(t, page.Next)

			// a page that ends with the last project doesn't have a next one.
			page, err = projectsDB.ListActiveWithOwners(ctx, console.ListProjectsWithOwnersParams{Limit: 1, After: afterB1})
			require.NoError(t, err)
			require.Equal(t, ids(projectC1), pageIDs(page))
			require.Nil(t, page.Next)

			// projects with the same owner and name are ordered by their public IDs, so each is listed once.
			sameName := insertProject(userA, projectA1.Name, console.ProjectActive)
			all := list(console.ListProjectsWithOwnersParams{})
			require.Len(t, all, 5)

			var paged []uuid.UUID
			params := console.ListProjectsWithOwnersParams{Limit: 1}
			for {
				page, err := projectsDB.ListActiveWithOwners(ctx, params)
				require.NoError(t, err)
				require.Len(t, page.Projects, 1)
				paged = append(paged, page.Projects[0].ID)
				if page.Next == nil {
					break
				}
				params.After = *page.Next
			}
			require.Equal(t, all, paged)
			require.Contains(t, paged, sameName.ID)

			// the list continues after the position even when the project at it is gone.
			require.NoError(t, projectsDB.Delete(ctx, projectB1.ID))
			page, err = projectsDB.ListActiveWithOwners(ctx, console.ListProjectsWithOwnersParams{Limit: 3, After: afterB1})
			require.NoError(t, err)
			require.Equal(t, ids(projectC1), pageIDs(page))

			_, err = projectsDB.ListActiveWithOwners(ctx, console.ListProjectsWithOwnersParams{})
			require.Error(t, err)
		})
	})
}
