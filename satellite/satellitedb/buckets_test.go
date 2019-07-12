// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestBuckets(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		bucketsDB := db.Buckets()

		var bucketInfo = []storj.Bucket{
			{
				ID:                  testrand.UUID(),
				Name:                "testbucket",
				ProjectID:           testrand.UUID(),
				PartnerID:           testrand.UUID(),
				Created:             time.Now(),
				PathCipher:          storj.EncAESGCM,
				DefaultSegmentsSize: int64(100),
			},
			{ // no partner ID
				ID:                  testrand.UUID(),
				Name:                "testbucket",
				ProjectID:           testrand.UUID(),
				Created:             time.Now(),
				PathCipher:          storj.EncAESGCM,
				DefaultSegmentsSize: int64(100),
			},
		}

		for _, info := range bucketInfo {
			_, err := bucketsDB.CreateBucket(ctx, info)
			require.Error(t, err)
		}
	})

}
