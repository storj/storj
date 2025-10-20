// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
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
