// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

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

		proj, err := projectsRepo.Insert(ctx, &console.Project{
			Name:    "Project",
			OwnerID: user1.ID,
		})
		require.NoError(t, err)

		_, err = projectMembers.Insert(ctx, user1.ID, proj.ID)
		require.NoError(t, err)

		projects, err := projectsRepo.GetByUserID(ctx, user1.ID)
		require.NoError(t, err)
		require.Len(t, projects, 1)
		require.Equal(t, 1, projects[0].MemberCount)

		_, err = projectMembers.Insert(ctx, user2.ID, proj.ID)
		require.NoError(t, err)

		projects, err = projectsRepo.GetByUserID(ctx, user1.ID)
		require.NoError(t, err)
		require.Len(t, projects, 1)
		require.Equal(t, 2, projects[0].MemberCount)
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
