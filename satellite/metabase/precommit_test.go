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
			for unversioned := range 3 {
				for _, highestVisible := range []bool{false, true} {
					name := fmt.Sprintf("Missing/Pending:%v,Unversioned:%v,HighestVisible:%v", pending, unversioned, highestVisible)
					t.Run(name, func(t *testing.T) {
						obj := metabasetest.RandObjectStream()

						info, err := precommit(metabase.PrecommitQuery{
							ObjectStream:    obj,
							Pending:         pending,
							Unversioned:     unversioned == 1,
							FullUnversioned: unversioned == 2,
							HighestVisible:  highestVisible,
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

		for i, tc := range []struct {
			Version                  metabase.Version
			WithoutExpiresAt         bool
			WithoutEncryptedUserData bool
		}{
			{Version: 12345, WithoutExpiresAt: true, WithoutEncryptedUserData: true},
			{Version: 12345, WithoutExpiresAt: false, WithoutEncryptedUserData: true},
			{Version: 12345, WithoutExpiresAt: true, WithoutEncryptedUserData: false},
			{Version: -12345, WithoutExpiresAt: true, WithoutEncryptedUserData: true},
			{Version: -12345, WithoutExpiresAt: false, WithoutEncryptedUserData: true},
			{Version: -12345, WithoutExpiresAt: true, WithoutEncryptedUserData: false},
		} {
			t.Run(fmt.Sprintf("pending-version-%d", i), func(t *testing.T) {
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
						ExpiresAt:         tc.WithoutExpiresAt,
						EncryptedUserData: tc.WithoutEncryptedUserData,
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
						CreatedAt:  pending.CreatedAt,
						Encryption: pending.Encryption,
					},
					Segments:       []metabase.PrecommitSegment{},
					HighestVisible: 0,
					Unversioned:    nil,
				}

				if !tc.WithoutExpiresAt {
					expect.Pending.ExpiresAt = pending.ExpiresAt
				}
				if !tc.WithoutEncryptedUserData {
					expect.Pending.EncryptedMetadata = encryptedUserData.EncryptedMetadata
					expect.Pending.EncryptedMetadataNonce = encryptedUserData.EncryptedMetadataNonce
					expect.Pending.EncryptedMetadataEncryptedKey = encryptedUserData.EncryptedMetadataEncryptedKey
					expect.Pending.EncryptedETag = encryptedUserData.EncryptedETag
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
					Version:  objectCommitted.Version,
					StreamID: objectCommitted.StreamID,
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
