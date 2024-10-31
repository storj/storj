// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
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

func TestGetUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.LiveAccounting.AsOfSystemInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		consoleDB := sat.DB.Console()

		consoleUser := &console.User{
			ID:               testrand.UUID(),
			FullName:         "Test User",
			Email:            "test@storj.io",
			PasswordHash:     testrand.Bytes(8),
			Status:           console.Inactive,
			UserAgent:        []byte("agent"),
			DefaultPlacement: 5,
		}

		_, apiErr := service.GetUserByEmail(ctx, consoleUser.Email)
		require.Equal(t, http.StatusNotFound, apiErr.Status)
		require.Error(t, apiErr.Err)

		_, err := consoleDB.Users().Insert(ctx, consoleUser)
		require.NoError(t, err)

		consoleUser.PaidTier = true
		require.NoError(
			t,
			sat.DB.Console().Users().Update(ctx, consoleUser.ID, console.UpdateUserRequest{PaidTier: &consoleUser.PaidTier}),
		)

		// User is deactivated, so it cannot be retrieved by e-mail.
		_, apiErr = service.GetUserByEmail(ctx, consoleUser.Email)
		require.Equal(t, http.StatusNotFound, apiErr.Status)
		require.Error(t, apiErr.Err)

		consoleUser.Status = console.Active
		require.NoError(
			t,
			consoleDB.Users().Update(ctx, consoleUser.ID, console.UpdateUserRequest{Status: &consoleUser.Status}),
		)

		user, apiErr := service.GetUserByEmail(ctx, consoleUser.Email)
		require.NoError(t, apiErr.Err)
		require.NotNil(t, user)
		require.Equal(t, consoleUser.ID, user.User.ID)
		require.Equal(t, consoleUser.FullName, user.User.FullName)
		require.Equal(t, consoleUser.Email, user.User.Email)
		require.Equal(t, consoleUser.PaidTier, user.PaidTier)
		require.Equal(t, consoleUser.Status.String(), user.Status)
		require.Equal(t, string(consoleUser.UserAgent), user.UserAgent)
		require.Equal(t, consoleUser.DefaultPlacement, user.DefaultPlacement)
		require.Empty(t, user.Projects)

		type expectedTotal struct {
			storage   int64
			segments  int64
			bandwidth int64
			objects   int64
		}

		var projects []*console.Project
		var expectedTotals []expectedTotal

		for projNum := 1; projNum <= 2; projNum++ {
			storageLimit := memory.GB * memory.Size(projNum)
			bandwidthLimit := memory.GB * memory.Size(projNum*2)
			segmentLimit := int64(1000 * projNum)

			proj := &console.Project{
				ID:             testrand.UUID(),
				Name:           fmt.Sprintf("Project %d", projNum),
				OwnerID:        consoleUser.ID,
				StorageLimit:   &storageLimit,
				BandwidthLimit: &bandwidthLimit,
				SegmentLimit:   &segmentLimit,
			}

			proj, err := consoleDB.Projects().Insert(ctx, proj)
			require.NoError(t, err)
			projects = append(projects, proj)

			_, err = consoleDB.ProjectMembers().Insert(ctx, user.User.ID, proj.ID, console.RoleAdmin)
			require.NoError(t, err)

			bucket, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      testrand.BucketName(),
				ProjectID: proj.ID,
			})
			require.NoError(t, err)

			total := expectedTotal{}

			for objNum := 0; objNum < projNum; objNum++ {
				obj := metabasetest.CreateObject(ctx, t, sat.Metabase.DB, metabase.ObjectStream{
					ProjectID:  proj.ID,
					BucketName: metabase.BucketName(bucket.Name),
					ObjectKey:  metabasetest.RandObjectKey(),
					Version:    12345,
					StreamID:   testrand.UUID(),
				}, byte(16*projNum))

				total.storage += obj.TotalEncryptedSize
				total.segments += int64(obj.SegmentCount)
				total.objects++
			}

			testBandwidth := int64(2000 * projNum)
			err = sat.DB.Orders().
				UpdateBucketBandwidthAllocation(ctx, proj.ID, []byte(bucket.Name), pb.PieceAction_GET, testBandwidth, time.Now())
			require.NoError(t, err)
			total.bandwidth += testBandwidth

			expectedTotals = append(expectedTotals, total)
		}

		sat.Accounting.Tally.Loop.TriggerWait()

		user, apiErr = service.GetUserByEmail(ctx, consoleUser.Email)
		require.NoError(t, apiErr.Err)
		require.NotNil(t, user)
		require.Len(t, user.Projects, len(projects))

		sort.Slice(user.Projects, func(i, j int) bool {
			return user.Projects[i].Name < user.Projects[j].Name
		})

		for i, info := range user.Projects {
			proj := projects[i]
			name := proj.Name
			require.Equal(t, proj.PublicID, info.ID, name)
			require.Equal(t, name, info.Name, name)
			require.EqualValues(t, *proj.StorageLimit, info.StorageLimit, name)
			require.EqualValues(t, *proj.BandwidthLimit, info.BandwidthLimit, name)
			require.Equal(t, *proj.SegmentLimit, info.SegmentLimit, name)

			total := expectedTotals[i]
			require.Equal(t, total.storage, *info.StorageUsed, name)
			require.Equal(t, total.bandwidth, info.BandwidthUsed, name)
			require.Equal(t, total.segments, *info.SegmentUsed, name)
		}
	})
}
