// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUsers(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		consoleDB := db.Console()

		// create user
		userPassHash := testrand.Bytes(8)

		// create an user with partnerID
		_, err := consoleDB.Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "John Doe",
			Email:        "john@mail.test",
			PasswordHash: userPassHash,
			Status:       console.Active,
			PartnerID:    testrand.UUID(),
		})
		require.NoError(t, err)

		// create an user with no partnerID
		_, err = consoleDB.Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "John Doe",
			Email:        "john@mail.test",
			PasswordHash: userPassHash,
			Status:       console.Active,
		})
		require.NoError(t, err)

		// create a project with partnerID
		_, err = consoleDB.Projects().Insert(ctx, &console.Project{
			ID:          testrand.UUID(),
			Name:        "John Doe",
			Description: "some description",
			PartnerID:   testrand.UUID(),
			CreatedAt:   time.Now(),
		})
		require.NoError(t, err)

		// create a project with no partnerID
		proj, err := consoleDB.Projects().Insert(ctx, &console.Project{
			ID:          testrand.UUID(),
			Name:        "John Doe",
			Description: "some description",
			CreatedAt:   time.Now(),
		})
		require.NoError(t, err)

		// create a APIKey with no partnerID
		_, err = consoleDB.APIKeys().Create(ctx, testrand.Bytes(8), console.APIKeyInfo{
			ID:        testrand.UUID(),
			ProjectID: proj.ID,
			Name:      "John Doe",
			Secret:    []byte("xyz"),
			CreatedAt: time.Now(),
		})
		require.NoError(t, err)

		// create a bucket with no partnerID
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:                  testrand.UUID(),
			Name:                "testbucket",
			ProjectID:           proj.ID,
			Created:             time.Now(),
			PathCipher:          storj.EncAESGCM,
			DefaultSegmentsSize: int64(100),
		})
		require.NoError(t, err)

		// update a bucket with partnerID
		bucket, err := db.Buckets().UpdateBucket(ctx, storj.Bucket{
			ID:                  testrand.UUID(),
			Name:                "testbucket",
			ProjectID:           proj.ID,
			PartnerID:           proj.ID,
			Created:             time.Now(),
			PathCipher:          storj.EncAESGCM,
			DefaultSegmentsSize: int64(100),
		})
		require.NoError(t, err)
		bucket, err = db.Buckets().GetBucket(ctx, []byte("testbucket"), proj.ID)
		require.NoError(t, err)
		flag := uuid.Equal(bucket.PartnerID, proj.ID)
		require.True(t, flag)
	})
}
