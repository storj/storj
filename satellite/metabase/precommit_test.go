// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
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
					err := db.ChooseAdapter(obj.Location().ProjectID).WithTx(ctx, metabase.TransactionOptions{}, func(ctx context.Context, adapter metabase.TransactionAdapter) error {
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
			err := adapter.WithTx(ctx, metabase.TransactionOptions{}, func(ctx context.Context, tx metabase.TransactionAdapter) error {
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
		precommit := func(loc metabase.ObjectLocation, bypassGovernance bool) (result metabase.PrecommitConstraintWithNonPendingResult, err error) {
			err = db.ChooseAdapter(loc.ProjectID).WithTx(ctx, metabase.TransactionOptions{}, func(ctx context.Context, tx metabase.TransactionAdapter) (err error) {
				result, err = tx.PrecommitDeleteUnversionedWithNonPending(ctx, metabase.PrecommitDeleteUnversionedWithNonPending{
					ObjectLocation: loc,
					ObjectLock: metabase.ObjectLockDeleteOptions{
						Enabled:          true,
						BypassGovernance: bypassGovernance,
					},
				})
				return
			})
			return
		}

		objStream := metabasetest.RandObjectStream()
		loc := objStream.Location()

		t.Run("No objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			result, err := precommit(loc, false)
			require.NoError(t, err)
			require.Empty(t, result)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		metabasetest.ObjectLockDeletionTestRunner{
			TestProtected: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj, segs := metabasetest.CreateTestObject{
					BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
						ObjectStream: objStream,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    testCase.Retention,
						LegalHold:    testCase.LegalHold,
					},
				}.Run(ctx, t, db, objStream, 3)

				result, err := precommit(loc, testCase.BypassGovernance)
				require.True(t, metabase.ErrObjectLock.Has(err))
				require.Empty(t, result)

				metabasetest.Verify{
					Objects:  []metabase.RawObject{metabase.RawObject(obj)},
					Segments: metabasetest.SegmentsToRaw(segs),
				}.Check(ctx, t, db)
			},
			TestRemovable: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				committed, _ := metabasetest.CreateTestObject{
					BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
						ObjectStream: objStream,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    testCase.Retention,
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

				result, err := precommit(loc, testCase.BypassGovernance)
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

func TestPrecommitQuery(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		precommit := func(query metabase.PrecommitQuery) (metabase.PrecommitInfo, error) {
			adapter := db.ChooseAdapter(query.ObjectStream.ProjectID)
			var info metabase.PrecommitInfo
			err := adapter.WithTx(ctx, metabase.TransactionOptions{}, func(ctx context.Context, tx metabase.TransactionAdapter) error {
				var err error
				info, err = db.PrecommitQuery(ctx, query, tx)
				return err
			})
			return info, err
		}

		for _, pending := range []bool{false, true} {
			for _, unversioned := range []bool{false, true} {
				for _, highestVisible := range []bool{false, true} {
					name := fmt.Sprintf("Missing/Pending:%v,Unversioned:%v,HighestVisible:%v", pending, unversioned, highestVisible)
					t.Run(name, func(t *testing.T) {
						obj := metabasetest.RandObjectStream()

						info, err := precommit(metabase.PrecommitQuery{
							ObjectStream:   obj,
							Pending:        pending,
							Unversioned:    unversioned,
							HighestVisible: highestVisible,
						})
						if pending {
							require.ErrorContains(t, err, "object with specified version and pending status is missing")
						} else {
							require.NoError(t, err)
						}

						expect := metabase.PrecommitInfo{
							TimestampVersion: info.TimestampVersion, // this is dynamically created
						}
						require.Equal(t, expect, info)
					})
				}
			}
		}

		t.Run("positive-pending", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()

			expiration := time.Now().Add(48 * time.Hour)
			encryptedUserData := metabasetest.RandEncryptedUserData()

			pending := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					ExpiresAt:    &expiration,
					Retention:    metabase.Retention{},
					LegalHold:    false,

					EncryptedUserData: encryptedUserData,
					Encryption:        metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			info, err := precommit(metabase.PrecommitQuery{
				Pending:        true,
				ObjectStream:   obj,
				Unversioned:    true,
				HighestVisible: true,
			})
			require.NoError(t, err)

			expect := metabase.PrecommitInfo{
				HighestVersion:   pending.Version,
				TimestampVersion: info.TimestampVersion,
				Pending: &metabase.PrecommitPendingObject{
					CreatedAt:                     pending.CreatedAt,
					ExpiresAt:                     pending.ExpiresAt,
					Encryption:                    pending.Encryption,
					EncryptedMetadata:             encryptedUserData.EncryptedMetadata,
					EncryptedMetadataNonce:        encryptedUserData.EncryptedMetadataNonce,
					EncryptedMetadataEncryptedKey: encryptedUserData.EncryptedMetadataEncryptedKey,
					EncryptedETag:                 encryptedUserData.EncryptedETag,
				},
				Segments:       []metabase.PrecommitSegment{},
				HighestVisible: 0,
				Unversioned:    nil,
			}

			require.EqualExportedValues(t, expect, info)
		})

		t.Run("negative-pending", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			obj.Version = -12345

			expiration := time.Now().Add(48 * time.Hour)
			encryptedUserData := metabasetest.RandEncryptedUserData()

			pending := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					ExpiresAt:    &expiration,
					Retention:    metabase.Retention{},
					LegalHold:    false,

					EncryptedUserData: encryptedUserData,
					Encryption:        metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			info, err := precommit(metabase.PrecommitQuery{
				Pending:        true,
				ObjectStream:   obj,
				Unversioned:    true,
				HighestVisible: true,
			})
			require.NoError(t, err)

			expect := metabase.PrecommitInfo{
				HighestVersion:   0, // we don't return negative versions
				TimestampVersion: info.TimestampVersion,
				Pending: &metabase.PrecommitPendingObject{
					CreatedAt:                     pending.CreatedAt,
					ExpiresAt:                     pending.ExpiresAt,
					Encryption:                    pending.Encryption,
					EncryptedMetadata:             encryptedUserData.EncryptedMetadata,
					EncryptedMetadataNonce:        encryptedUserData.EncryptedMetadataNonce,
					EncryptedMetadataEncryptedKey: encryptedUserData.EncryptedMetadataEncryptedKey,
					EncryptedETag:                 encryptedUserData.EncryptedETag,
				},
				Segments:       []metabase.PrecommitSegment{},
				HighestVisible: 0,
				Unversioned:    nil,
			}

			require.EqualExportedValues(t, expect, info)
		})

		t.Run("existing-unversioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()

			expiration := time.Now().Add(48 * time.Hour)
			encryptedUserData := metabasetest.RandEncryptedUserData()

			pending := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					ExpiresAt:    &expiration,
					Retention:    metabase.Retention{},
					LegalHold:    false,

					EncryptedUserData: encryptedUserData,
					Encryption:        metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			objCommitted := obj
			objCommitted.StreamID = testrand.UUID()
			objCommitted.Version = 20000
			objectCommitted := metabasetest.CreateObject(ctx, t, db, objCommitted, 2)

			info, err := precommit(metabase.PrecommitQuery{
				Pending:        true,
				ObjectStream:   obj,
				Unversioned:    true,
				HighestVisible: true,
			})
			require.NoError(t, err)

			expect := metabase.PrecommitInfo{
				HighestVersion:   20000,
				TimestampVersion: info.TimestampVersion,
				Pending: &metabase.PrecommitPendingObject{
					CreatedAt:                     pending.CreatedAt,
					ExpiresAt:                     pending.ExpiresAt,
					Encryption:                    pending.Encryption,
					EncryptedMetadata:             encryptedUserData.EncryptedMetadata,
					EncryptedMetadataNonce:        encryptedUserData.EncryptedMetadataNonce,
					EncryptedMetadataEncryptedKey: encryptedUserData.EncryptedMetadataEncryptedKey,
					EncryptedETag:                 encryptedUserData.EncryptedETag,
				},
				Segments:       []metabase.PrecommitSegment{},
				HighestVisible: metabase.CommittedUnversioned,
				Unversioned: &metabase.PrecommitUnversionedObject{
					Version:      objectCommitted.Version,
					StreamID:     objectCommitted.StreamID,
					SegmentCount: 2,
				},
			}

			require.EqualExportedValues(t, expect, info)
		})

		t.Run("existing-versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()

			expiration := time.Now().Add(48 * time.Hour)
			encryptedUserData := metabasetest.RandEncryptedUserData()

			pending := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					ExpiresAt:    &expiration,
					Retention:    metabase.Retention{},
					LegalHold:    false,

					EncryptedUserData: encryptedUserData,
					Encryption:        metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			objCommitted := obj
			objCommitted.StreamID = testrand.UUID()
			objCommitted.Version = 20000
			metabasetest.CreateObjectVersioned(ctx, t, db, objCommitted, 2)

			info, err := precommit(metabase.PrecommitQuery{
				Pending:        true,
				ObjectStream:   obj,
				Unversioned:    true,
				HighestVisible: true,
			})
			require.NoError(t, err)

			expect := metabase.PrecommitInfo{
				HighestVersion:   20000,
				TimestampVersion: info.TimestampVersion,
				Pending: &metabase.PrecommitPendingObject{
					CreatedAt:                     pending.CreatedAt,
					ExpiresAt:                     pending.ExpiresAt,
					Encryption:                    pending.Encryption,
					EncryptedMetadata:             encryptedUserData.EncryptedMetadata,
					EncryptedMetadataNonce:        encryptedUserData.EncryptedMetadataNonce,
					EncryptedMetadataEncryptedKey: encryptedUserData.EncryptedMetadataEncryptedKey,
					EncryptedETag:                 encryptedUserData.EncryptedETag,
				},
				Segments:       []metabase.PrecommitSegment{},
				HighestVisible: metabase.CommittedVersioned,
				Unversioned:    nil,
			}

			require.EqualExportedValues(t, expect, info)
		})
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
				err := adapter.WithTx(ctx, metabase.TransactionOptions{}, func(ctx context.Context, adapter metabase.TransactionAdapter) error {
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
				err := adapter.WithTx(ctx, metabase.TransactionOptions{}, func(ctx context.Context, adapter metabase.TransactionAdapter) error {
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
	metabasetest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db *metabase.DB) {
		baseObj := metabasetest.RandObjectStream()

		adapter := db.ChooseAdapter(baseObj.ProjectID)

		var objects []metabase.RawObject
		for i := 0; i < b.N; i++ {
			baseObj.ObjectKey = metabase.ObjectKey(fmt.Sprintf("overwrite/%d", i))
			object := metabase.RawObject{
				ObjectStream: baseObj,
				Status:       metabase.CommittedUnversioned,
			}
			objects = append(objects, object)
		}
		err := db.TestingBatchInsertObjects(ctx, objects)
		require.NoError(b, err)
		b.ResetTimer()

		b.Run("nooverwrite", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				objectKey := metabase.ObjectKey(fmt.Sprintf("nooverwrite/%d", i))
				err := adapter.WithTx(ctx, metabase.TransactionOptions{}, func(ctx context.Context, adapter metabase.TransactionAdapter) error {
					_, err := db.PrecommitConstraint(ctx, metabase.PrecommitConstraint{
						Location: metabase.ObjectLocation{
							ProjectID:  baseObj.ProjectID,
							BucketName: baseObj.BucketName,
							ObjectKey:  objectKey,
						},
						Versioned:      false,
						DisallowDelete: false,
					}, adapter)
					return err
				})
				require.NoError(b, err)
			}
		})

		b.Run("overwrite", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				objectKey := metabase.ObjectKey(fmt.Sprintf("overwrite/%d", i))
				err := adapter.WithTx(ctx, metabase.TransactionOptions{}, func(ctx context.Context, adapter metabase.TransactionAdapter) error {
					_, err := db.PrecommitConstraint(ctx, metabase.PrecommitConstraint{
						Location: metabase.ObjectLocation{
							ProjectID:  baseObj.ProjectID,
							BucketName: baseObj.BucketName,
							ObjectKey:  objectKey,
						},
						Versioned:      false,
						DisallowDelete: false,
					}, adapter)
					return err
				})
				require.NoError(b, err)
			}
		})
	})
}
