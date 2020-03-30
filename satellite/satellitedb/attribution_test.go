// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUsers(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		consoleDB := db.Console()

		// create user
		userPassHash := testrand.Bytes(8)

		// create an user with partnerID
		_, err := consoleDB.Users().Insert(ctx, &console.User{
			ID:           testrand.UUID2(),
			FullName:     "John Doe",
			Email:        "john@mail.test",
			PasswordHash: userPassHash,
			Status:       console.Active,
			PartnerID:    testrand.UUID2(),
		})
		require.NoError(t, err)

		// create an user with no partnerID
		_, err = consoleDB.Users().Insert(ctx, &console.User{
			ID:           testrand.UUID2(),
			FullName:     "John Doe",
			Email:        "john@mail.test",
			PasswordHash: userPassHash,
			Status:       console.Active,
		})
		require.NoError(t, err)

		// create a project with partnerID
		_, err = consoleDB.Projects().Insert(ctx, &console.Project{
			ID:          testrand.UUID2(),
			Name:        "John Doe",
			Description: "some description",
			PartnerID:   testrand.UUID2(),
			CreatedAt:   time.Now(),
		})
		require.NoError(t, err)

		// create a project with no partnerID
		proj, err := consoleDB.Projects().Insert(ctx, &console.Project{
			ID:          testrand.UUID2(),
			Name:        "John Doe",
			Description: "some description",
			CreatedAt:   time.Now(),
		})
		require.NoError(t, err)

		// create a APIKey with no partnerID
		_, err = consoleDB.APIKeys().Create(ctx, testrand.Bytes(8), console.APIKeyInfo{
			ID:        testrand.UUID2(),
			ProjectID: proj.ID,
			Name:      "John Doe",
			Secret:    []byte("xyz"),
			CreatedAt: time.Now(),
		})
		require.NoError(t, err)

		// create a bucket with no partnerID
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:                  storj.DeprecatedUUID(testrand.UUID2()),
			Name:                "testbucket",
			ProjectID:           storj.DeprecatedUUID(proj.ID),
			Created:             time.Now(),
			PathCipher:          storj.EncAESGCM,
			DefaultSegmentsSize: int64(100),
		})
		require.NoError(t, err)

		// update a bucket with partnerID
		bucket, err := db.Buckets().UpdateBucket(ctx, storj.Bucket{
			ID:                  storj.DeprecatedUUID(testrand.UUID2()),
			Name:                "testbucket",
			ProjectID:           storj.DeprecatedUUID(proj.ID),
			PartnerID:           storj.DeprecatedUUID(proj.ID),
			Created:             time.Now(),
			PathCipher:          storj.EncAESGCM,
			DefaultSegmentsSize: int64(100),
		})
		require.NoError(t, err)
		bucket, err = db.Buckets().GetBucket(ctx, []byte("testbucket"), proj.ID)
		require.NoError(t, err)
		flag := uuid.UUID(bucket.PartnerID) == proj.ID
		require.True(t, flag)
	})
}
