// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"testing"
	"time"

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

// TestFinishCopyObjectUnversionedConcurrentCopies reproduces two concurrent
// unversioned copies to the same location leaving two committed unversioned
// objects behind (the "multiple committed unversioned objects" error).
//
// On TiDB a copy used to insert its pending object in a separate statement
// with a positive version (highest+1):
//
//  1. Copy C1 inserts pending v1. Its commit transaction finds no committed
//     unversioned object and updates v1 to committed, but has not committed yet.
//  2. Copy C2 inserts pending v2. Its unversioned check looks only at committed
//     statuses, so it skips C1's still-pending v1 and does not wait for C1.
//     C2 commits v2, then C1 commits v1.
//
// Now the pending copy gets a negative version, so C1's v1 is C2's highest
// version and C2 waits for C1's lock.
//
// C1 is emulated with the old statements in a raw transaction, so the
// interleaving is deterministic. C2 is the real FinishCopyObject.
func TestFinishCopyObjectUnversionedConcurrentCopies(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		if db.Implementation() != dbutil.TiDB {
			t.Skip("TiDB only: pending copy objects get a positive version in a separate statement")
		}
		defer metabasetest.DeleteAll{}.Check(ctx, t, db)

		rawDB := db.ChooseAdapter(testrand.UUID()).(*metabase.TiDBAdapter).UnderlyingDB()

		// Test databases have a unique index that production does not have. It
		// turns this race into a duplicate key error and a retry, hiding it.
		_, err := rawDB.ExecContext(ctx, `ALTER TABLE objects DROP INDEX objects_one_unversioned_per_location`)
		require.NoError(t, err)

		source := metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 0)

		target := metabasetest.RandObjectStream()
		target.ProjectID = source.ProjectID // a copy stays within the project
		target.Version = 1
		location := []any{target.ProjectID.Bytes(), target.BucketName, target.ObjectKey}

		// C1: pending copy object with a positive version, as the old insertPendingCopyObject inserted it.
		require.NoError(t, db.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
			ObjectStream: target,
			CreatedAt:    time.Now(),
			Status:       metabase.Pending,
			Encryption:   metabasetest.DefaultEncryption,
		}}))

		// C1: commit transaction up to, but not including, COMMIT.
		conn, err := rawDB.Conn(ctx)
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		c1, err := conn.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = c1.Rollback() }()

		var highest int64
		require.NoError(t, c1.QueryRowContext(ctx, `
			SELECT version FROM objects
			WHERE (project_id, bucket_name, object_key) = (?, ?, ?) AND version > 0
			ORDER BY version DESC LIMIT 1
			FOR UPDATE`, location...).Scan(&highest))
		require.EqualValues(t, 1, highest)

		var unversionedCount int
		require.NoError(t, c1.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM (
				SELECT version FROM objects
				WHERE (project_id, bucket_name, object_key) = (?, ?, ?) AND version > 0
					AND status IN (3, 6)
				FOR UPDATE
			) AS unversioned`, location...).Scan(&unversionedCount))
		require.Zero(t, unversionedCount)

		_, err = c1.ExecContext(ctx, `
			UPDATE objects SET status = ?, zombie_deletion_deadline = NULL
			WHERE (project_id, bucket_name, object_key, version) = (?, ?, ?, ?)
				AND status = ?`,
			append([]any{metabase.CommittedUnversioned}, append(location, target.Version, metabase.Pending)...)...)
		require.NoError(t, err)

		// C2: a second unversioned copy to the same location.
		c2Done := make(chan error, 1)
		go func() {
			c2Ctx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			_, err := db.FinishCopyObject(c2Ctx, metabase.FinishCopyObject{
				ObjectStream:          source.ObjectStream,
				NewBucket:             target.BucketName,
				NewEncryptedObjectKey: target.ObjectKey,
				NewStreamID:           testrand.UUID(),
				NewVersioned:          false,
			})
			c2Done <- err
		}()

		// Commit C1 once C2 has finished (the bug) or is waiting for C1's locks
		// (a correct implementation).
		var c2Err error
		c2Finished := false
		require.Eventually(t, func() bool {
			select {
			case c2Err = <-c2Done:
				c2Finished = true
				return true
			default:
			}
			var waits int
			err := rawDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.data_lock_waits`).Scan(&waits)
			return err == nil && waits > 0
		}, 30*time.Second, 10*time.Millisecond, "C2 neither finished nor waited for C1")

		require.NoError(t, c1.Commit())
		if !c2Finished {
			c2Err = <-c2Done
		}
		require.NoError(t, c2Err)

		objects, err := db.TestingAllObjects(ctx)
		require.NoError(t, err)

		var unversioned []metabase.Object
		for _, object := range objects {
			if object.Location() != target.Location() {
				continue
			}
			if object.Status == metabase.CommittedUnversioned || object.Status == metabase.DeleteMarkerUnversioned {
				unversioned = append(unversioned, object)
			}
		}
		require.Len(t, unversioned, 1, "expected exactly one committed unversioned object, got: %+v", unversioned)
	}, metabasetest.WithTimestampVersioning)
}
