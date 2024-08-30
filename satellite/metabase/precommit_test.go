// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestPrecommitConstraint_Empty(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		for _, versioned := range []bool{false, true} {
			for _, disallowDelete := range []bool{false, true} {
				name := fmt.Sprintf("Versioned:%v,DisallowDelete:%v", versioned, disallowDelete)
				t.Run(name, func(t *testing.T) {
					var result metabase.PrecommitConstraintResult
					err := db.ChooseAdapter(obj.Location().ProjectID).WithTx(ctx, func(ctx context.Context, adapter metabase.TransactionAdapter) error {
						var err error
						result, err = db.PrecommitConstraint(ctx, metabase.PrecommitConstraint{
							Location:       obj.Location(),
							Versioned:      versioned,
							DisallowDelete: disallowDelete,
						}, adapter)
						return err
					})
					require.NoError(t, err)
					require.Equal(t, metabase.PrecommitConstraintResult{}, result)
				})
			}
		}

		t.Run("with-non-pending", func(t *testing.T) {
			adapter := db.ChooseAdapter(obj.ProjectID)
			var result metabase.PrecommitConstraintWithNonPendingResult
			err := adapter.WithTx(ctx, func(ctx context.Context, tx metabase.TransactionAdapter) error {
				var err error
				result, err = tx.PrecommitDeleteUnversionedWithNonPending(ctx, metabase.PrecommitDeleteUnversionedWithNonPending{
					ObjectLocation: obj.Location(),
				})
				return err
			})
			require.NoError(t, err)
			require.Equal(t, metabase.PrecommitConstraintWithNonPendingResult{}, result)
		})
	})
}

func TestDeleteUnversionedWithNonPendingUsingObjectLock(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		precommit := func(loc metabase.ObjectLocation) (result metabase.PrecommitConstraintWithNonPendingResult, err error) {
			err = db.ChooseAdapter(loc.ProjectID).WithTx(ctx, func(ctx context.Context, tx metabase.TransactionAdapter) (err error) {
				result, err = tx.PrecommitDeleteUnversionedWithNonPending(ctx, metabase.PrecommitDeleteUnversionedWithNonPending{
					ObjectLocation:    loc,
					ObjectLockEnabled: true,
				})
				return
			})
			return
		}

		objStream := metabasetest.RandObjectStream()
		loc := objStream.Location()

		t.Run("No objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			result, err := precommit(loc)
			require.NoError(t, err)
			require.Empty(t, result)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		objectLockTestRunner{
			TestActive: func(t *testing.T, retention metabase.Retention, legalHold bool) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj, segs := metabasetest.CreateTestObject{
					BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
						ObjectStream: objStream,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    retention,
						LegalHold:    legalHold,
					},
				}.Run(ctx, t, db, objStream, 3)

				result, err := precommit(loc)
				require.True(t, metabase.ErrObjectLock.Has(err))
				require.Empty(t, result)

				metabasetest.Verify{
					Objects:  []metabase.RawObject{metabase.RawObject(obj)},
					Segments: metabasetest.SegmentsToRaw(segs),
				}.Check(ctx, t, db)
			},
			TestExpired: func(t *testing.T, retention metabase.Retention) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				committed, _ := metabasetest.CreateTestObject{
					BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
						ObjectStream: objStream,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    retention,
					},
				}.Run(ctx, t, db, objStream, 3)

				pendingObjStream := objStream
				pendingObjStream.Version++
				pending := metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: pendingObjStream,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				result, err := precommit(loc)
				require.NoError(t, err)
				require.Equal(t, metabase.PrecommitConstraintWithNonPendingResult{
					Deleted:                  []metabase.Object{committed},
					DeletedObjectCount:       1,
					DeletedSegmentCount:      3,
					HighestVersion:           pending.Version,
					HighestNonPendingVersion: committed.Version,
				}, result)
			},
		}.Run(t)
	})
}

func BenchmarkPrecommitConstraint(b *testing.B) {
	metabasetest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db *metabase.DB) {
		baseObj := metabasetest.RandObjectStream()

		for i := 0; i < 500; i++ {
			metabasetest.CreateObject(ctx, b, db, metabasetest.RandObjectStream(), 0)
		}

		for i := 0; i < 10; i++ {
			baseObj.ObjectKey = metabase.ObjectKey("foo/" + strconv.Itoa(i))
			metabasetest.CreateObject(ctx, b, db, baseObj, 0)

			baseObj.ObjectKey = metabase.ObjectKey("foo/prefixA/" + strconv.Itoa(i))
			metabasetest.CreateObject(ctx, b, db, baseObj, 0)

			baseObj.ObjectKey = metabase.ObjectKey("foo/prefixB/" + strconv.Itoa(i))
			metabasetest.CreateObject(ctx, b, db, baseObj, 0)
		}

		for i := 0; i < 50; i++ {
			baseObj.ObjectKey = metabase.ObjectKey("boo/foo" + strconv.Itoa(i) + "/object")
			metabasetest.CreateObject(ctx, b, db, baseObj, 0)
		}

		adapter := db.ChooseAdapter(baseObj.ProjectID)
		b.Run("unversioned", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := adapter.WithTx(ctx, func(ctx context.Context, adapter metabase.TransactionAdapter) error {
					_, err := db.PrecommitConstraint(ctx, metabase.PrecommitConstraint{
						Location: metabase.ObjectLocation{
							ProjectID:  baseObj.ProjectID,
							BucketName: baseObj.BucketName,
							ObjectKey:  "foo/5",
						},
						Versioned:      false,
						DisallowDelete: false,
					}, adapter)
					return err
				})
				require.NoError(b, err)
			}
		})

		b.Run("versioned", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := adapter.WithTx(ctx, func(ctx context.Context, adapter metabase.TransactionAdapter) error {
					_, err := db.PrecommitConstraint(ctx, metabase.PrecommitConstraint{
						Location: metabase.ObjectLocation{
							ProjectID:  baseObj.ProjectID,
							BucketName: baseObj.BucketName,
							ObjectKey:  "foo/5",
						},
						Versioned:      true,
						DisallowDelete: false,
					}, adapter)
					return err
				})
				require.NoError(b, err)
			}
		})
	})
}

func BenchmarkPrecommitConstraintUnversioned(b *testing.B) {
	for _, precommitDeleteMode := range metabase.PrecommitDeleteModes {
		metabasetest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db *metabase.DB) {
			baseObj := metabasetest.RandObjectStream()

			adapter := db.ChooseAdapter(baseObj.ProjectID)

			var objects []metabase.RawObject
			for i := 0; i < b.N; i++ {
				baseObj.ObjectKey = metabase.ObjectKey(fmt.Sprintf("overwrite/%d", i))
				object := metabase.RawObject{
					ObjectStream: baseObj,
				}
				objects = append(objects, object)
			}
			err := db.TestingBatchInsertObjects(ctx, objects)
			require.NoError(b, err)
			b.ResetTimer()

			b.Run(fmt.Sprintf("nooverwrite_%d", precommitDeleteMode), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					objectKey := metabase.ObjectKey(fmt.Sprintf("nooverwrite/%d", i))
					err := adapter.WithTx(ctx, func(ctx context.Context, adapter metabase.TransactionAdapter) error {
						_, err := db.PrecommitConstraint(ctx, metabase.PrecommitConstraint{
							Location: metabase.ObjectLocation{
								ProjectID:  baseObj.ProjectID,
								BucketName: baseObj.BucketName,
								ObjectKey:  objectKey,
							},
							Versioned:                  false,
							DisallowDelete:             false,
							TestingPrecommitDeleteMode: precommitDeleteMode,
						}, adapter)
						return err
					})
					require.NoError(b, err)
				}
			})

			b.Run(fmt.Sprintf("overwrite_%d", precommitDeleteMode), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					objectKey := metabase.ObjectKey(fmt.Sprintf("overwrite/%d", i))
					err := adapter.WithTx(ctx, func(ctx context.Context, adapter metabase.TransactionAdapter) error {
						_, err := db.PrecommitConstraint(ctx, metabase.PrecommitConstraint{
							Location: metabase.ObjectLocation{
								ProjectID:  baseObj.ProjectID,
								BucketName: baseObj.BucketName,
								ObjectKey:  objectKey,
							},
							Versioned:                  false,
							DisallowDelete:             false,
							TestingPrecommitDeleteMode: precommitDeleteMode,
						}, adapter)
						return err
					})
					require.NoError(b, err)
				}
			})
		})
	}
}
