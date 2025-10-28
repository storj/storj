// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

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
		precommit := func(query metabase.PrecommitQuery) (*metabase.PrecommitInfo, error) {
			adapter := db.ChooseAdapter(query.ObjectStream.ProjectID)
			var info *metabase.PrecommitInfo
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
							require.Nil(t, info)
						} else {
							require.NoError(t, err)
							expect := &metabase.PrecommitInfo{
								ObjectStream:     obj,
								TimestampVersion: info.TimestampVersion, // this is dynamically created
							}
							require.Equal(t, expect, info)
						}
					})

				}
			}
		}

		for _, tc := range []struct {
			Version          metabase.Version
			WithoutExpiresAt bool
		}{{12345, true}, {-12345, true}, {12345, false}, {-12345, false}} {
			label := "positive"
			if tc.Version < 0 {
				label = "negative"
			}

			t.Run(fmt.Sprintf("pending-version-%s-without-expires-at-%v", label, tc.WithoutExpiresAt), func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj := metabasetest.RandObjectStream()
				obj.Version = tc.Version

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
					Pending: true,
					ExcludeFromPending: metabase.ExcludeFromPending{
						ExpiresAt: tc.WithoutExpiresAt,
					},
					ObjectStream:   obj,
					Unversioned:    true,
					HighestVisible: true,
				})
				require.NoError(t, err)

				expectedVersion := pending.Version
				if tc.Version < 0 {
					expectedVersion = 0 // we don't return negative versions
				}

				expect := &metabase.PrecommitInfo{
					ObjectStream:     obj,
					HighestVersion:   expectedVersion,
					TimestampVersion: info.TimestampVersion,
					Pending: &metabase.PrecommitPendingObject{
						CreatedAt:                     pending.CreatedAt,
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

				if !tc.WithoutExpiresAt {
					expect.Pending.ExpiresAt = pending.ExpiresAt
				}

				require.EqualExportedValues(t, expect, info)
			})
		}

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

			expect := &metabase.PrecommitInfo{
				ObjectStream:     obj,
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

			expect := &metabase.PrecommitInfo{
				ObjectStream:     obj,
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
