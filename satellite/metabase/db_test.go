// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/dbutil/pgutil/pgerrcode"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestNow(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		sysnow := time.Now()
		now, err := db.Now(ctx)
		require.NoError(t, err)
		require.WithinDuration(t, sysnow, now, 5*time.Second)
	})
}

func TestDisallowDoubleUnversioned(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// This checks that TestingUniqueUnversioned=true indeed works as needed.
		objStream := metabasetest.RandObjectStream()
		obj := metabasetest.CreateObject(ctx, t, db, objStream, 0)

		internaldb := db.UnderlyingTagSQL()
		_, err := internaldb.Exec(ctx, `
			INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				status
			) VALUES (
				$1, $2, $3, $4, $5,
				`+strconv.Itoa(int(metabase.CommittedUnversioned))+`
			)
		`, obj.ProjectID, []byte(obj.BucketName), obj.ObjectKey, obj.Version+1, testrand.UUID(),
		)
		require.True(t, pgerrcode.IsConstraintViolation(err))
		require.ErrorContains(t, err, "objects_one_unversioned_per_location")

		_, err = internaldb.Exec(ctx, `
			INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				status
			) VALUES (
				$1, $2, $3, $4, $5,
				`+strconv.Itoa(int(metabase.DeleteMarkerUnversioned))+`
			)
		`, obj.ProjectID, []byte(obj.BucketName), obj.ObjectKey, obj.Version+1, testrand.UUID(),
		)
		require.True(t, pgerrcode.IsConstraintViolation(err))
		require.ErrorContains(t, err, "objects_one_unversioned_per_location")

		metabasetest.Verify{
			Objects: []metabase.RawObject{
				metabase.RawObject(obj),
			},
		}.Check(ctx, t, db)
	})
}
