// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestGetProject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.LiveAccounting.AsOfSystemInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		t.Run("unexisting project", func(t *testing.T) {
			sat := planet.Satellites[0]

			service := sat.Admin.Admin.Service
			_, apiErr := service.GetProject(ctx, testrand.UUID())
			require.Error(t, apiErr.Err)
			assert.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("existing user", func(t *testing.T) {
			consoleUser := &console.User{
				ID:               testrand.UUID(),
				FullName:         "Test User",
				Email:            "test@storj.io",
				PasswordHash:     testrand.Bytes(8),
				Status:           console.Inactive,
				UserAgent:        []byte("agent"),
				DefaultPlacement: 5,
			}

			sat := planet.Satellites[0]
			consoleDB := sat.DB.Console()
			_, err := consoleDB.Users().Insert(ctx, consoleUser)
			require.NoError(t, err)

			consoleUser.Status = console.Active
			require.NoError(
				t,
				consoleDB.Users().Update(ctx, consoleUser.ID, console.UpdateUserRequest{Status: &consoleUser.Status}),
			)

			// Project with default limits
			// Note: the projects DB Insert method will set storage, segment, bandwidth limits to whatever is passed in.
			// If nothing is passed, they are nil. In production, a project is created by the console service, which calls
			// this DB method with the default limits passed.
			projID := testrand.UUID()
			consoleProject := &console.Project{
				ID:             projID,
				Name:           "project-free-account",
				Description:    "This is a project created at the time that owner's user account is a free account",
				OwnerID:        consoleUser.ID,
				StorageLimit:   &sat.Config.Console.UsageLimits.Storage.Free,
				BandwidthLimit: &sat.Config.Console.UsageLimits.Bandwidth.Free,
				SegmentLimit:   &sat.Config.Console.UsageLimits.Segment.Free,
			}

			consoleProject, err = consoleDB.Projects().Insert(ctx, consoleProject)
			require.NoError(t, err)
			projPublicID := consoleProject.PublicID

			service := sat.Admin.Admin.Service
			project, apiErr := service.GetProject(ctx, projPublicID)
			require.NoError(t, apiErr.Err)

			assert.Equal(t, projPublicID, project.ID)
			assert.Equal(t, consoleProject.Name, project.Name)
			assert.Equal(t, consoleProject.Description, project.Description)
			assert.EqualValues(t, consoleProject.UserAgent, project.UserAgent)
			assert.Equal(t, consoleProject.CreatedAt, project.CreatedAt)

			// service should return defaults for these since they are null in DB.
			defaultRate := int(sat.Config.Metainfo.RateLimiter.Rate)
			defaultBurst := defaultRate
			defaultMaxBuckets := sat.Config.Metainfo.ProjectLimits.MaxBuckets

			// check DB value is null and admin value is not null.
			require.Nil(t, consoleProject.RateLimit)
			require.NotNil(t, project.RateLimit)
			assert.Equal(t, defaultRate, *project.RateLimit)
			require.Nil(t, consoleProject.BurstLimit)
			require.NotNil(t, project.BurstLimit)
			assert.Equal(t, defaultBurst, *project.BurstLimit)
			require.Nil(t, consoleProject.MaxBuckets)
			require.NotNil(t, project.MaxBuckets)
			assert.Equal(t, defaultMaxBuckets, *project.MaxBuckets)

			assert.Equal(t, consoleProject.OwnerID, project.Owner.ID)
			assert.Equal(t, consoleUser.FullName, project.Owner.FullName)
			assert.Equal(t, consoleUser.Email, project.Owner.Email)

			assert.EqualValues(t, consoleProject.BandwidthLimit, project.BandwidthLimit)
			assert.Zero(t, project.BandwidthUsed)
			assert.EqualValues(t, consoleProject.StorageLimit, project.StorageLimit)
			require.NotNil(t, project.StorageUsed)
			assert.Zero(t, *project.StorageUsed)
			assert.EqualValues(t, consoleProject.SegmentLimit, project.SegmentLimit)
			require.NotNil(t, project.SegmentUsed)
			assert.Zero(t, *project.SegmentUsed)

			// now set the null columns to specific values and check admin returns them.
			newBucketLimit := defaultMaxBuckets * 2
			require.NoError(t, consoleDB.Projects().UpdateBucketLimit(ctx, projID, newBucketLimit))

			newRateLimit := defaultRate * 2
			require.NoError(t, consoleDB.Projects().UpdateRateLimit(ctx, projID, &newRateLimit))

			// set burst to different value than rate to make sure they are returned correctly (burst == rate by default)
			newBurstLimit := defaultBurst * 3
			require.NoError(t, consoleDB.Projects().UpdateBurstLimit(ctx, projID, &newBurstLimit))

			project, apiErr = service.GetProject(ctx, projPublicID)
			require.NoError(t, apiErr.Err)

			require.NotNil(t, project.MaxBuckets)
			require.Equal(t, newBucketLimit, *project.MaxBuckets)
			require.NotNil(t, project.RateLimit)
			require.Equal(t, newRateLimit, *project.RateLimit)
			require.NotNil(t, project.BurstLimit)
			require.Equal(t, newBurstLimit, *project.BurstLimit)

			// Create usage
			bucket, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      testrand.BucketName(),
				ProjectID: projID,
			})
			require.NoError(t, err)

			obj := metabasetest.CreateObject(ctx, t, sat.Metabase.DB, metabase.ObjectStream{
				ProjectID:  projID,
				BucketName: bucket.Name,
				ObjectKey:  metabasetest.RandObjectKey(),
				Version:    12345,
				StreamID:   testrand.UUID(),
			}, 16)

			usedStorage := obj.TotalEncryptedSize
			usedSegments := int64(obj.SegmentCount)

			usedBandwidth := int64(2000)
			err = sat.DB.Orders().
				UpdateBucketBandwidthAllocation(ctx, projID, []byte(bucket.Name), pb.PieceAction_GET, usedBandwidth, time.Now())
			require.NoError(t, err)
			sat.Accounting.Tally.Loop.TriggerWait()

			project, apiErr = service.GetProject(ctx, projPublicID)
			require.NoError(t, apiErr.Err)

			assert.Equal(t, usedBandwidth, project.BandwidthUsed)
			require.NotNil(t, project.StorageUsed)
			assert.Equal(t, usedStorage, *project.StorageUsed)
			require.NotNil(t, project.SegmentUsed)
			assert.Equal(t, usedSegments, *project.SegmentUsed)
		})
	})
}
