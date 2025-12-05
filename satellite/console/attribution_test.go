// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
)

func TestUsers(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		bucketService := sat.API.Buckets.Service
		db := sat.DB
		consoleDB := db.Console()

		// create user
		userPassHash := testrand.Bytes(8)

		// create an user with no UserAgent
		_, err := consoleDB.Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "John Doe",
			Email:        "john@mail.test",
			PasswordHash: userPassHash,
			Status:       console.Active,
		})
		require.NoError(t, err)

		// create a project with UserAgent
		testUserAgent := []byte("test user agent")
		_, err = consoleDB.Projects().Insert(ctx, &console.Project{
			ID:          testrand.UUID(),
			Name:        "John Doe",
			Description: "some description",
			CreatedAt:   time.Now(),
			UserAgent:   testUserAgent,
		})
		require.NoError(t, err)

		// create a project with no UserAgent
		proj, err := consoleDB.Projects().Insert(ctx, &console.Project{
			ID:          testrand.UUID(),
			Name:        "John Doe",
			Description: "some description",
			CreatedAt:   time.Now(),
		})
		require.NoError(t, err)

		// create a APIKey with no UserAgent
		_, err = consoleDB.APIKeys().Create(ctx, testrand.Bytes(8), console.APIKeyInfo{
			ID:        testrand.UUID(),
			ProjectID: proj.ID,
			Name:      "John Doe",
			Secret:    []byte("xyz"),
			CreatedAt: time.Now(),
		})
		require.NoError(t, err)

		// create a bucket with no UserAgent
		_, err = bucketService.CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "testbucket",
			ProjectID: proj.ID,
			Created:   time.Now(),
		})
		require.NoError(t, err)

		// update a bucket with UserAgent
		bucket, err := bucketService.UpdateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "testbucket",
			ProjectID: proj.ID,
			Created:   time.Now(),
			UserAgent: testUserAgent,
		})
		require.NoError(t, err)
		require.Equal(t, testUserAgent, bucket.UserAgent)

		bucket, err = bucketService.GetBucket(ctx, []byte("testbucket"), proj.ID)
		require.NoError(t, err)
		require.Equal(t, testUserAgent, bucket.UserAgent)
	})
}
