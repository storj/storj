// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
)

// TestVersionConflictConcurrency exercises the operations that compute the
// next object version and insert it in a single statement. On TiDB two
// concurrent computations can pick the same version and the losing insert
// fails with a duplicate primary key error unless it is retried.
func TestVersionConflictConcurrency(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		if db.Implementation() != dbutil.TiDB {
			t.Skip("regression test for the TiDB adapter's version conflict retry")
		}

		const workers = 8

		t.Run("BeginObjectNextVersion", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			var group errgroup.Group
			for w := 0; w < workers; w++ {
				group.Go(func() error {
					for i := 0; i < 5; i++ {
						stream := obj
						stream.Version = metabase.NextVersion
						stream.StreamID = testrand.UUID()
						_, err := db.BeginObjectNextVersion(ctx, metabase.BeginObjectNextVersion{
							ObjectStream: stream,
							Encryption:   metabasetest.DefaultEncryption,
						})
						if err != nil {
							return err
						}
					}
					return nil
				})
			}
			require.NoError(t, group.Wait())
		})

		t.Run("DeleteObjectLastCommittedVersioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := metabasetest.RandObjectStream()
			location := stream.Location()
			var group errgroup.Group
			for w := 0; w < workers; w++ {
				group.Go(func() error {
					for i := 0; i < 5; i++ {
						_, err := db.DeleteObjectLastCommitted(ctx, metabase.DeleteObjectLastCommitted{
							ObjectLocation: location,
							Versioned:      true,
						})
						if err != nil {
							return err
						}
					}
					return nil
				})
			}
			require.NoError(t, group.Wait())
		})

		t.Run("FinishCopyObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			source := metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 0)
			target := metabasetest.RandObjectStream()
			var group errgroup.Group
			for w := 0; w < workers; w++ {
				group.Go(func() error {
					for i := 0; i < 3; i++ {
						_, err := db.FinishCopyObject(ctx, metabase.FinishCopyObject{
							ObjectStream:          source.ObjectStream,
							NewBucket:             target.BucketName,
							NewEncryptedObjectKey: target.ObjectKey,
							NewStreamID:           testrand.UUID(),
							NewVersioned:          true,
						})
						if err != nil {
							return err
						}
					}
					return nil
				})
			}
			require.NoError(t, group.Wait())
		})
	})
}
