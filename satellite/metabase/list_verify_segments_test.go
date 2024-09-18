// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestListBucketsStreamIDs(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("many objects segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			nbBuckets := 3
			bucketList := metabase.ListVerifyBucketList{}
			obj := metabasetest.RandObjectStream()
			for i := 0; i < nbBuckets; i++ {
				projectID := testrand.UUID()
				projectID[0] = byte(i) // make projectID ordered
				bucketName := metabase.BucketName(testrand.BucketName())
				bucketList.Add(projectID, bucketName)

				obj.ProjectID = projectID
				obj.BucketName = bucketName
				obj.StreamID[0] = byte(i) // make StreamIDs ordered
				_ = metabasetest.CreateObject(ctx, t, db, obj, 3)
				// create a un-related object
				_ = metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 2)

			}

			opts := metabase.ListBucketsStreamIDs{
				BucketList: bucketList,

				Limit: 10,
			}
			listStreamIDsResult, err := db.ListBucketsStreamIDs(ctx, opts)
			require.NoError(t, err)
			require.Len(t, listStreamIDsResult.StreamIDs, nbBuckets)
			require.Equal(t, obj.ProjectID, listStreamIDsResult.LastBucket.ProjectID)
			require.Equal(t, obj.BucketName, listStreamIDsResult.LastBucket.BucketName)
			require.Equal(t, obj.StreamID,
				listStreamIDsResult.StreamIDs[len(listStreamIDsResult.StreamIDs)-1])
			// TODO more test cases
		})
	})
}
