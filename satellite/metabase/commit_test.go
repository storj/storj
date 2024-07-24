// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"math"
	"testing"
	"time"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
)

func TestBeginObjectNextVersion(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		objectStream := metabase.ObjectStream{
			ProjectID:  obj.ProjectID,
			BucketName: obj.BucketName,
			ObjectKey:  obj.ObjectKey,
			StreamID:   obj.StreamID,
		}

		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream: test.ObjectStream,
						Encryption:   metabasetest.DefaultEncryption,
					},
					Version:  -1,
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("invalid EncryptedMetadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:      objectStream,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedMetadata: testrand.BytesInt(32),
				},
				Version:  -1,
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set if EncryptedMetadata is set",
			}.Check(ctx, t, db)

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:                  objectStream,
					Encryption:                    metabasetest.DefaultEncryption,
					EncryptedMetadataEncryptedKey: testrand.BytesInt(32),
				},
				Version:  -1,
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be not set if EncryptedMetadata is not set",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("disallow exact version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objectStream.Version = 5

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version:  -1,
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version should be metabase.NextVersion",
			}.Check(ctx, t, db)
		})

		t.Run("NextVersion", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()
			zombieDeadline := now1.Add(24 * time.Hour)
			futureTime := now1.Add(10 * 24 * time.Hour)

			objectStream.Version = metabase.NextVersion

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			now2 := time.Now()

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           objectStream,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &futureTime,
				},
				Version: 2,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    1,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    2,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now2,
						Status:    metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &futureTime,
					},
				},
			}.Check(ctx, t, db)
		})

		// TODO: expires at date
		// TODO: zombie deletion deadline

		t.Run("Retention", func(t *testing.T) {
			t.Run("Success", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()
				zombieDeadline := now.Add(24 * time.Hour)

				retention := metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: now.Add(time.Minute),
				}

				metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream: objectStream,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    retention,
					},
					Version: 1,
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  objectStream.ProjectID,
							BucketName: objectStream.BucketName,
							ObjectKey:  objectStream.ObjectKey,
							Version:    1,
							StreamID:   objectStream.StreamID,
						},
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
						Retention:              retention,
					}},
				}.Check(ctx, t, db)
			})

			t.Run("Invalid retention configuration", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				check := func(mode storj.RetentionMode, retainUntil time.Time, errText string) {
					metabasetest.BeginObjectNextVersion{
						Opts: metabase.BeginObjectNextVersion{
							ObjectStream: objectStream,
							Encryption:   metabasetest.DefaultEncryption,
							Retention: metabase.Retention{
								Mode:        mode,
								RetainUntil: retainUntil,
							},
						},
						Version:  1,
						ErrClass: &metabase.ErrInvalidRequest,
						ErrText:  errText,
					}.Check(ctx, t, db)
				}

				now := time.Now()

				check(storj.ComplianceMode, time.Time{}, "retention period expiration must be set if retention mode is set")
				check(storj.NoRetention, now.Add(time.Minute), "retention period expiration must not be set if retention mode is not set")
				check(storj.RetentionMode(2), now.Add(time.Minute), "retention mode must be 0 (none) or 1 (compliance), but it was 2")

				metabasetest.Verify{}.Check(ctx, t, db)
			})

			t.Run("Retention configuration with TTL", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()
				expires := now.Add(time.Minute)

				metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream: objectStream,
						Encryption:   metabasetest.DefaultEncryption,
						Retention: metabase.Retention{
							Mode:        storj.ComplianceMode,
							RetainUntil: now.Add(time.Minute),
						},
						ExpiresAt: &expires,
					},
					Version:  1,
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "ExpiresAt must not be set if Retention is set",
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		})

		t.Run("older committed version exists", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objectStream.Version = metabase.NextVersion

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    1,
						StreamID:   obj.StreamID,
					},
				},
			}.Check(ctx, t, db)

			now2 := time.Now()
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 2,
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    2,
						StreamID:   obj.StreamID,
					},
				},
			}.Check(ctx, t, db)

			// currently CommitObject always deletes previous versions so only version 2 left
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    2,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now2,
						Status:    metabase.CommittedUnversioned,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("newer committed version exists", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()

			objectStream.Version = metabase.NextVersion

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 2,
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    2,
						StreamID:   obj.StreamID,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    1,
						StreamID:   obj.StreamID,
					},
				},
				ExpectVersion: 3,
			}.Check(ctx, t, db)

			// currently CommitObject always deletes previous versions so only version 1 left
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    3,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.CommittedUnversioned,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("begin object next version with metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			zombieDeadline := now.Add(24 * time.Hour)

			objectStream.Version = metabase.NextVersion

			encryptedMetadata := testrand.BytesInt(64)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataEncryptedKey := testrand.BytesInt(32)

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,

					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadataEncryptedKey: encryptedMetadataEncryptedKey,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    metabase.DefaultVersion,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now,
						Status:    metabase.Pending,

						EncryptedMetadata:             encryptedMetadata,
						EncryptedMetadataNonce:        encryptedMetadataNonce[:],
						EncryptedMetadataEncryptedKey: encryptedMetadataEncryptedKey,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestBeginObjectExactVersion(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: test.ObjectStream,
						Encryption:   metabasetest.DefaultEncryption,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		objectStream := metabase.ObjectStream{
			ProjectID:  obj.ProjectID,
			BucketName: obj.BucketName,
			ObjectKey:  obj.ObjectKey,
			StreamID:   obj.StreamID,
		}

		t.Run("invalid EncryptedMetadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objectStream.Version = 1

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:      objectStream,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedMetadata: testrand.BytesInt(32),
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set if EncryptedMetadata is set",
			}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:                  objectStream,
					Encryption:                    metabasetest.DefaultEncryption,
					EncryptedMetadataEncryptedKey: testrand.BytesInt(32),
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be not set if EncryptedMetadata is not set",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("disallow NextVersion", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objectStream.Version = metabase.NextVersion

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version should not be metabase.NextVersion",
			}.Check(ctx, t, db)
		})

		t.Run("Specific version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()
			zombieDeadline := now1.Add(24 * time.Hour)

			objectStream.Version = 5

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Duplicate pending version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()
			zombieDeadline := now1.Add(24 * time.Hour)

			objectStream.Version = 5

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				ErrClass: &metabase.ErrObjectAlreadyExists,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Duplicate committed version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()

			objectStream.Version = 5

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream,
				},
			}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				ErrClass: &metabase.ErrObjectAlreadyExists,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.CommittedUnversioned,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})
		// TODO: expires at date
		// TODO: zombie deletion deadline

		t.Run("Retention", func(t *testing.T) {
			objectStream.Version = 5

			t.Run("Success", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()
				zombieDeadline := now.Add(24 * time.Hour)

				retention := metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: now.Add(time.Minute),
				}

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: objectStream,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    retention,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{{
						ObjectStream:           objectStream,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
						Retention:              retention,
					}},
				}.Check(ctx, t, db)
			})

			t.Run("Invalid retention configuration", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				check := func(mode storj.RetentionMode, retainUntil time.Time, errText string) {
					metabasetest.BeginObjectExactVersion{
						Opts: metabase.BeginObjectExactVersion{
							ObjectStream: objectStream,
							Encryption:   metabasetest.DefaultEncryption,
							Retention: metabase.Retention{
								Mode:        mode,
								RetainUntil: retainUntil,
							},
						},
						ErrClass: &metabase.ErrInvalidRequest,
						ErrText:  errText,
					}.Check(ctx, t, db)
				}

				now := time.Now()

				check(storj.ComplianceMode, time.Time{}, "retention period expiration must be set if retention mode is set")
				check(storj.NoRetention, now.Add(time.Minute), "retention period expiration must not be set if retention mode is not set")
				check(storj.RetentionMode(2), now.Add(time.Minute), "retention mode must be 0 (none) or 1 (compliance), but it was 2")

				metabasetest.Verify{}.Check(ctx, t, db)
			})

			t.Run("Retention configuration with TTL", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()
				expires := now.Add(time.Minute)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: objectStream,
						Encryption:   metabasetest.DefaultEncryption,
						Retention: metabase.Retention{
							Mode:        storj.ComplianceMode,
							RetainUntil: now.Add(time.Minute),
						},
						ExpiresAt: &expires,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "ExpiresAt must not be set if Retention is set",
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		})

		t.Run("Older committed version exists", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()

			objectStream.Version = 100

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream,
				},
			}.Check(ctx, t, db)

			objectStream.Version = 300

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream,
				},
			}.Check(ctx, t, db)

			// currently CommitObject always deletes previous versions so only version 3 left
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    300,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.CommittedUnversioned,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Newer committed version exists", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()

			objectStream.Version = 300

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream,
				},
			}.Check(ctx, t, db)

			objectStream.Version = 100

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream,
				},
				ExpectVersion: 301,
			}.Check(ctx, t, db)

			// currently CommitObject always deletes previous versions so only version 1 left
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    301,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.CommittedUnversioned,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("begin object exact version with metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			zombieDeadline := now.Add(24 * time.Hour)

			objectStream.Version = 100

			encryptedMetadata := testrand.BytesInt(64)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataEncryptedKey := testrand.BytesInt(32)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,

					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadataEncryptedKey: encryptedMetadataEncryptedKey,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  objectStream.ProjectID,
							BucketName: objectStream.BucketName,
							ObjectKey:  objectStream.ObjectKey,
							Version:    objectStream.Version,
							StreamID:   objectStream.StreamID,
						},
						CreatedAt: now,
						Status:    metabase.Pending,

						EncryptedMetadata:             encryptedMetadata,
						EncryptedMetadataNonce:        encryptedMetadataNonce[:],
						EncryptedMetadataEncryptedKey: encryptedMetadataEncryptedKey,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestBeginSegment(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.BeginSegment{
					Opts: metabase.BeginSegment{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("RootPieceID missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "RootPieceID missing",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Pieces missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					RootPieceID:  storj.PieceID{1},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "pieces missing",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("StorageNode in pieces missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: storj.NodeID{},
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "piece number 1 is missing storage node id",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Piece number 2 is duplicated", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "duplicated piece number 1",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Pieces should be ordered", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{
						{
							Number:      2,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "pieces should be ordered",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("pending object missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					RootPieceID:  storj.PieceID{1},
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.ErrPendingObjectMissing,
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("pending object missing when object committed", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			now := time.Now()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					RootPieceID:  storj.PieceID{1},
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.ErrPendingObjectMissing,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("begin segment successfully", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			now := time.Now()
			zombieDeadline := now.Add(24 * time.Hour)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					RootPieceID:  storj.PieceID{1},
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("multiple begin segment successfully", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			now := time.Now()
			zombieDeadline := now.Add(24 * time.Hour)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			for i := 0; i < 5; i++ {
				metabasetest.BeginSegment{
					Opts: metabase.BeginSegment{
						ObjectStream: obj,
						RootPieceID:  storj.PieceID{1},
						Pieces: []metabase.Piece{{
							Number:      1,
							StorageNode: testrand.NodeID(),
						}},
					},
				}.Check(ctx, t, db)
			}

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestCommitSegment(t *testing.T) {
	for _, mode := range []string{"", "transaction", "no-pending-object-check"} {
		mode := mode
		metabasetest.RunWithConfig(t, metabase.Config{
			ApplicationName:          "metabase-tests",
			TestingCommitSegmentMode: mode,
		}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
			obj := metabasetest.RandObjectStream()
			now := time.Now()

			for _, test := range metabasetest.InvalidObjectStreams(obj) {
				test := test
				t.Run(test.Name, func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					metabasetest.CommitSegment{
						Opts: metabase.CommitSegment{
							ObjectStream: test.ObjectStream,
						},
						ErrClass: test.ErrClass,
						ErrText:  test.ErrText,
					}.Check(ctx, t, db)
					metabasetest.Verify{}.Check(ctx, t, db)
				})
			}

			t.Run("invalid request", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()
				zombieDeadline := now.Add(24 * time.Hour)
				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Pieces: metabase.Pieces{{
							Number:      1,
							StorageNode: testrand.NodeID(),
						}},
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "RootPieceID missing",
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "pieces missing",
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Pieces: []metabase.Piece{{
							Number:      1,
							StorageNode: storj.NodeID{},
						}},
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "piece number 1 is missing storage node id",
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Pieces: []metabase.Piece{
							{
								Number:      1,
								StorageNode: testrand.NodeID(),
							},
							{
								Number:      1,
								StorageNode: testrand.NodeID(),
							},
						},
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "duplicated piece number 1",
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Pieces: []metabase.Piece{
							{
								Number:      2,
								StorageNode: testrand.NodeID(),
							},
							{
								Number:      1,
								StorageNode: testrand.NodeID(),
							},
						},
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "pieces should be ordered",
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						RootPieceID:  testrand.PieceID(),
						Pieces: metabase.Pieces{{
							Number:      1,
							StorageNode: testrand.NodeID(),
						}},
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "EncryptedKey missing",
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						RootPieceID:  testrand.PieceID(),

						Pieces: metabase.Pieces{{
							Number:      1,
							StorageNode: testrand.NodeID(),
						}},

						EncryptedKey: testrand.Bytes(32),
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "EncryptedKeyNonce missing",
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						RootPieceID:  testrand.PieceID(),

						Pieces: metabase.Pieces{{
							Number:      1,
							StorageNode: testrand.NodeID(),
						}},

						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),

						EncryptedSize: -1,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "EncryptedSize negative or zero",
				}.Check(ctx, t, db)

				if metabase.ValidatePlainSize {
					metabasetest.CommitSegment{
						Opts: metabase.CommitSegment{
							ObjectStream: obj,
							RootPieceID:  testrand.PieceID(),

							Pieces: metabase.Pieces{{
								Number:      1,
								StorageNode: testrand.NodeID(),
							}},

							EncryptedKey:      testrand.Bytes(32),
							EncryptedKeyNonce: testrand.Bytes(32),

							EncryptedSize: 1024,
							PlainSize:     -1,
						},
						ErrClass: &metabase.ErrInvalidRequest,
						ErrText:  "PlainSize negative or zero",
					}.Check(ctx, t, db)
				}

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						RootPieceID:  testrand.PieceID(),

						Pieces: metabase.Pieces{{
							Number:      1,
							StorageNode: testrand.NodeID(),
						}},

						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   -1,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "PlainOffset negative",
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						RootPieceID:  testrand.PieceID(),

						Pieces: metabase.Pieces{{
							Number:      1,
							StorageNode: testrand.NodeID(),
						}},

						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "Redundancy zero",
				}.Check(ctx, t, db)

				redundancy := storj.RedundancyScheme{
					OptimalShares: 2,
				}

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Pieces: []metabase.Piece{
							{
								Number:      1,
								StorageNode: testrand.NodeID(),
							},
						},
						RootPieceID:       testrand.PieceID(),
						Redundancy:        redundancy,
						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "number of pieces is less than redundancy optimal shares value",
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.Pending,

							Encryption:             metabasetest.DefaultEncryption,
							ZombieDeletionDeadline: &zombieDeadline,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("duplicate", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now1 := time.Now()
				zombieDeadline := now1.Add(24 * time.Hour)
				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				rootPieceID := testrand.PieceID()
				pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)

				metabasetest.BeginSegment{
					Opts: metabase.BeginSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: 0, Index: 0},
						RootPieceID:  rootPieceID,
						Pieces:       pieces,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: 0, Index: 0},
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
						Redundancy:    metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: 0, Index: 0},
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
						Redundancy:    metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now1,
							Status:       metabase.Pending,

							Encryption:             metabasetest.DefaultEncryption,
							ZombieDeletionDeadline: &zombieDeadline,
						},
					},
					Segments: []metabase.RawSegment{
						{
							StreamID:  obj.StreamID,
							Position:  metabase.SegmentPosition{Part: 0, Index: 0},
							CreatedAt: now,

							RootPieceID:       rootPieceID,
							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							EncryptedSize: 1024,
							PlainOffset:   0,
							PlainSize:     512,

							Redundancy: metabasetest.DefaultRedundancy,

							Pieces: pieces,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("overwrite", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now1 := time.Now()
				zombieDeadline := now1.Add(24 * time.Hour)
				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				rootPieceID1 := testrand.PieceID()
				rootPieceID2 := testrand.PieceID()
				pieces1 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
				pieces2 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)

				metabasetest.BeginSegment{
					Opts: metabase.BeginSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: 0, Index: 0},
						RootPieceID:  rootPieceID1,
						Pieces:       pieces1,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: 0, Index: 0},
						RootPieceID:  rootPieceID1,
						Pieces:       pieces1,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
						Redundancy:    metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: 0, Index: 0},
						RootPieceID:  rootPieceID2,
						Pieces:       pieces2,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
						Redundancy:    metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now1,
							Status:       metabase.Pending,

							Encryption:             metabasetest.DefaultEncryption,
							ZombieDeletionDeadline: &zombieDeadline,
						},
					},
					Segments: []metabase.RawSegment{
						{
							StreamID:  obj.StreamID,
							Position:  metabase.SegmentPosition{Part: 0, Index: 0},
							CreatedAt: now,

							RootPieceID:       rootPieceID2,
							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							EncryptedSize: 1024,
							PlainOffset:   0,
							PlainSize:     512,

							Redundancy: metabasetest.DefaultRedundancy,

							Pieces: pieces2,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit segment of missing object", func(t *testing.T) {
				if mode == "no-pending-object-check" {
					t.Skip()
				}

				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				rootPieceID := testrand.PieceID()
				pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
						Redundancy:    metabasetest.DefaultRedundancy,
					},
					ErrClass: &metabase.ErrPendingObjectMissing,
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})

			t.Run("commit segment of committed object", func(t *testing.T) {
				if mode == "no-pending-object-check" {
					t.Skip()
				}

				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				rootPieceID := testrand.PieceID()
				pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)

				now := time.Now()
				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
						Redundancy:    metabasetest.DefaultRedundancy,
					},
					ErrClass: &metabase.ErrPendingObjectMissing,
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.CommittedUnversioned,

							Encryption: metabasetest.DefaultEncryption,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit segment of object with expires at", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				rootPieceID := testrand.PieceID()
				pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)

				now := time.Now()
				expectedExpiresAt := now.Add(33 * time.Hour)
				zombieDeadline := now.Add(24 * time.Hour)
				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
						ExpiresAt:    &expectedExpiresAt,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						ExpiresAt:    &expectedExpiresAt,
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
						Redundancy:    metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							ExpiresAt:    &expectedExpiresAt,
							Status:       metabase.Pending,

							Encryption:             metabasetest.DefaultEncryption,
							ZombieDeletionDeadline: &zombieDeadline,
						},
					},
					Segments: []metabase.RawSegment{
						{
							StreamID:  obj.StreamID,
							CreatedAt: now,
							ExpiresAt: &expectedExpiresAt,

							RootPieceID:       rootPieceID,
							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							EncryptedSize: 1024,
							PlainOffset:   0,
							PlainSize:     512,

							Redundancy: metabasetest.DefaultRedundancy,

							Pieces: pieces,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit segment of pending object", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				rootPieceID := testrand.PieceID()
				pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)
				encryptedETag := testrand.Bytes(32)

				now := time.Now()
				zombieDeadline := now.Add(24 * time.Hour)
				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
						Redundancy:    metabasetest.DefaultRedundancy,
						EncryptedETag: encryptedETag,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.Pending,

							Encryption:             metabasetest.DefaultEncryption,
							ZombieDeletionDeadline: &zombieDeadline,
						},
					},
					Segments: []metabase.RawSegment{
						{
							StreamID:  obj.StreamID,
							CreatedAt: now,

							RootPieceID:       rootPieceID,
							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							EncryptedSize: 1024,
							PlainOffset:   0,
							PlainSize:     512,
							EncryptedETag: encryptedETag,

							Redundancy: metabasetest.DefaultRedundancy,

							Pieces: pieces,
						},
					},
				}.Check(ctx, t, db)
			})
		})
	}
}

func TestCommitInlineSegment(t *testing.T) {
	for _, mode := range []string{"", "transaction", "no-pending-object-check"} {
		mode := mode
		metabasetest.RunWithConfig(t, metabase.Config{
			ApplicationName:          "metabase-tests",
			TestingCommitSegmentMode: mode,
		}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
			obj := metabasetest.RandObjectStream()
			for _, test := range metabasetest.InvalidObjectStreams(obj) {
				test := test
				t.Run(test.Name, func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					metabasetest.CommitInlineSegment{
						Opts: metabase.CommitInlineSegment{
							ObjectStream: test.ObjectStream,
						},
						ErrClass: test.ErrClass,
						ErrText:  test.ErrText,
					}.Check(ctx, t, db)
					metabasetest.Verify{}.Check(ctx, t, db)
				})
			}

			t.Run("invalid request", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "EncryptedKey missing",
				}.Check(ctx, t, db)

				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,
						InlineData:   []byte{1, 2, 3},
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "EncryptedKey missing",
				}.Check(ctx, t, db)

				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,

						InlineData: []byte{1, 2, 3},

						EncryptedKey: testrand.Bytes(32),
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "EncryptedKeyNonce missing",
				}.Check(ctx, t, db)

				if metabase.ValidatePlainSize {
					metabasetest.CommitInlineSegment{
						Opts: metabase.CommitInlineSegment{
							ObjectStream: obj,

							InlineData: []byte{1, 2, 3},

							EncryptedKey:      testrand.Bytes(32),
							EncryptedKeyNonce: testrand.Bytes(32),

							PlainSize: -1,
						},
						ErrClass: &metabase.ErrInvalidRequest,
						ErrText:  "PlainSize negative or zero",
					}.Check(ctx, t, db)
				}

				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,

						InlineData: []byte{1, 2, 3},

						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),

						PlainSize:   512,
						PlainOffset: -1,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "PlainOffset negative",
				}.Check(ctx, t, db)
			})

			t.Run("commit inline segment of missing object", func(t *testing.T) {
				if mode == "no-pending-object-check" {
					t.Skip()
				}

				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)

				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,
						InlineData:   []byte{1, 2, 3},

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainSize:   512,
						PlainOffset: 0,
					},
					ErrClass: &metabase.ErrPendingObjectMissing,
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})

			t.Run("duplicate", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()
				zombieDeadline := now.Add(24 * time.Hour)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)

				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: 0, Index: 0},
						InlineData:   []byte{1, 2, 3},

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainSize:   512,
						PlainOffset: 0,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: 0, Index: 0},
						InlineData:   []byte{1, 2, 3},

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainSize:   512,
						PlainOffset: 0,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.Pending,

							Encryption:             metabasetest.DefaultEncryption,
							ZombieDeletionDeadline: &zombieDeadline,
						},
					},
					Segments: []metabase.RawSegment{
						{
							StreamID:  obj.StreamID,
							Position:  metabase.SegmentPosition{Part: 0, Index: 0},
							CreatedAt: now,

							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							PlainOffset: 0,
							PlainSize:   512,

							InlineData:    []byte{1, 2, 3},
							EncryptedSize: 3,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("overwrite", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()
				zombieDeadline := now.Add(24 * time.Hour)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)

				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: 0, Index: 0},
						InlineData:   []byte{1, 2, 3},

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainSize:   512,
						PlainOffset: 0,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: 0, Index: 0},
						InlineData:   []byte{4, 5, 6},

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainSize:   512,
						PlainOffset: 0,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.Pending,

							Encryption:             metabasetest.DefaultEncryption,
							ZombieDeletionDeadline: &zombieDeadline,
						},
					},
					Segments: []metabase.RawSegment{
						{
							StreamID:  obj.StreamID,
							Position:  metabase.SegmentPosition{Part: 0, Index: 0},
							CreatedAt: now,

							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							PlainOffset: 0,
							PlainSize:   512,

							InlineData:    []byte{4, 5, 6},
							EncryptedSize: 3,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit segment of committed object", func(t *testing.T) {
				if mode == "no-pending-object-check" {
					t.Skip()
				}

				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)

				now := time.Now()

				metabasetest.CreateObject(ctx, t, db, obj, 0)
				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,
						InlineData:   []byte{1, 2, 3},

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainSize:   512,
						PlainOffset: 0,
					},
					ErrClass: &metabase.ErrPendingObjectMissing,
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.CommittedUnversioned,
							Encryption:   metabasetest.DefaultEncryption,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit empty segment of pending object", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)
				encryptedETag := testrand.Bytes(32)

				now := time.Now()
				zombieDeadline := now.Add(24 * time.Hour)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainSize:     0,
						PlainOffset:   0,
						EncryptedETag: encryptedETag,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream:           obj,
							CreatedAt:              now,
							Status:                 metabase.Pending,
							Encryption:             metabasetest.DefaultEncryption,
							ZombieDeletionDeadline: &zombieDeadline,
						},
					},
					Segments: []metabase.RawSegment{
						{
							StreamID:  obj.StreamID,
							CreatedAt: now,

							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							PlainOffset: 0,
							PlainSize:   0,

							EncryptedSize: 0,
							EncryptedETag: encryptedETag,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit segment of pending object", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)
				encryptedETag := testrand.Bytes(32)

				now := time.Now()
				zombieDeadline := now.Add(24 * time.Hour)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,
						InlineData:   []byte{1, 2, 3},

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainSize:     512,
						PlainOffset:   0,
						EncryptedETag: encryptedETag,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream:           obj,
							CreatedAt:              now,
							Status:                 metabase.Pending,
							Encryption:             metabasetest.DefaultEncryption,
							ZombieDeletionDeadline: &zombieDeadline,
						},
					},
					Segments: []metabase.RawSegment{
						{
							StreamID:  obj.StreamID,
							CreatedAt: now,

							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							PlainOffset: 0,
							PlainSize:   512,

							InlineData:    []byte{1, 2, 3},
							EncryptedSize: 3,
							EncryptedETag: encryptedETag,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit segment of object with expires at", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)
				encryptedETag := testrand.Bytes(32)

				now := time.Now()
				zombieDeadline := now.Add(24 * time.Hour)
				expectedExpiresAt := now.Add(33 * time.Hour)
				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
						ExpiresAt:    &expectedExpiresAt,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,
						ExpiresAt:    &expectedExpiresAt,
						InlineData:   []byte{1, 2, 3},

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainSize:     512,
						PlainOffset:   0,
						EncryptedETag: encryptedETag,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream:           obj,
							CreatedAt:              now,
							ExpiresAt:              &expectedExpiresAt,
							Status:                 metabase.Pending,
							Encryption:             metabasetest.DefaultEncryption,
							ZombieDeletionDeadline: &zombieDeadline,
						},
					},
					Segments: []metabase.RawSegment{
						{
							StreamID:  obj.StreamID,
							CreatedAt: now,
							ExpiresAt: &expectedExpiresAt,

							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							PlainOffset: 0,
							PlainSize:   512,

							InlineData:    []byte{1, 2, 3},
							EncryptedSize: 3,
							EncryptedETag: encryptedETag,
						},
					},
				}.Check(ctx, t, db)
			})
		})
	}
}

func TestCommitObject(t *testing.T) {
	for _, precommitMode := range metabase.PrecommitDeleteModes {
		metabasetest.RunWithConfig(t, metabase.Config{
			ApplicationName:            "metabase-tests",
			TestingPrecommitDeleteMode: precommitMode,
			MaxNumberOfParts:           10000,
		}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
			obj := metabasetest.RandObjectStream()

			for _, test := range metabasetest.InvalidObjectStreams(obj) {
				test := test
				t.Run(test.Name, func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					metabasetest.CommitObject{
						Opts: metabase.CommitObject{
							ObjectStream: test.ObjectStream,
						},
						ErrClass: test.ErrClass,
						ErrText:  test.ErrText,
					}.Check(ctx, t, db)
					metabasetest.Verify{}.Check(ctx, t, db)
				})
			}

			t.Run("invalid EncryptedMetadata", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    metabase.DefaultVersion,
							StreamID:   obj.StreamID,
						},
						OverrideEncryptedMetadata: true,
						EncryptedMetadata:         testrand.BytesInt(32),
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set if EncryptedMetadata is set",
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    metabase.DefaultVersion,
							StreamID:   obj.StreamID,
						},
						OverrideEncryptedMetadata:     true,
						EncryptedMetadataEncryptedKey: testrand.BytesInt(32),
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be not set if EncryptedMetadata is not set",
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})

			t.Run("version without pending", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
					},
					ErrClass: &metabase.ErrObjectNotFound,
					ErrText:  "metabase: object with specified version and pending status is missing", // TODO: this error message could be better
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})

			t.Run("version", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						Encryption: metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)
				now := time.Now()

				encryptedMetadata := testrand.Bytes(1024)
				encryptedMetadataNonce := testrand.Nonce()
				encryptedMetadataKey := testrand.Bytes(265)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						OverrideEncryptedMetadata:     true,
						EncryptedMetadataNonce:        encryptedMetadataNonce[:],
						EncryptedMetadata:             encryptedMetadata,
						EncryptedMetadataEncryptedKey: encryptedMetadataKey,
					},
				}.Check(ctx, t, db)

				// disallow for double commit
				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
					},
					ErrClass: &metabase.ErrObjectNotFound,
					ErrText:  "metabase: object with specified version and pending status is missing", // TODO: this error message could be better
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  obj.ProjectID,
								BucketName: obj.BucketName,
								ObjectKey:  obj.ObjectKey,
								Version:    5,
								StreamID:   obj.StreamID,
							},
							CreatedAt: now,
							Status:    metabase.CommittedUnversioned,

							EncryptedMetadataNonce:        encryptedMetadataNonce[:],
							EncryptedMetadata:             encryptedMetadata,
							EncryptedMetadataEncryptedKey: encryptedMetadataKey,

							Encryption: metabasetest.DefaultEncryption,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("disallow delete but nothing to delete", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						Encryption: metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)
				now := time.Now()

				encryptedMetadata := testrand.Bytes(1024)
				encryptedMetadataNonce := testrand.Nonce()
				encryptedMetadataKey := testrand.Bytes(265)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						OverrideEncryptedMetadata:     true,
						EncryptedMetadataNonce:        encryptedMetadataNonce[:],
						EncryptedMetadata:             encryptedMetadata,
						EncryptedMetadataEncryptedKey: encryptedMetadataKey,
						DisallowDelete:                true,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  obj.ProjectID,
								BucketName: obj.BucketName,
								ObjectKey:  obj.ObjectKey,
								Version:    5,
								StreamID:   obj.StreamID,
							},
							CreatedAt: now,
							Status:    metabase.CommittedUnversioned,

							EncryptedMetadataNonce:        encryptedMetadataNonce[:],
							EncryptedMetadata:             encryptedMetadata,
							EncryptedMetadataEncryptedKey: encryptedMetadataKey,

							Encryption: metabasetest.DefaultEncryption,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("disallow delete when committing unversioned", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				unversionedStream := obj
				unversionedStream.Version = 3
				unversionedObject := metabasetest.CreateObject(ctx, t, db, unversionedStream, 0)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						Encryption: metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)
				now := time.Now()
				zombieDeadline := now.Add(24 * time.Hour)

				encryptedMetadata := testrand.Bytes(1024)
				encryptedMetadataNonce := testrand.Nonce()
				encryptedMetadataKey := testrand.Bytes(265)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						EncryptedMetadataNonce:        encryptedMetadataNonce[:],
						EncryptedMetadata:             encryptedMetadata,
						EncryptedMetadataEncryptedKey: encryptedMetadataKey,
						DisallowDelete:                true,
					},
					ErrClass: &metabase.ErrPermissionDenied,
					ErrText:  "no permissions to delete existing object",
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						metabase.RawObject(unversionedObject),
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  obj.ProjectID,
								BucketName: obj.BucketName,
								ObjectKey:  obj.ObjectKey,
								Version:    5,
								StreamID:   obj.StreamID,
							},
							CreatedAt: now,
							Status:    metabase.Pending,

							Encryption:             metabasetest.DefaultEncryption,
							ZombieDeletionDeadline: &zombieDeadline,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("assign plain_offset", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)
				now := time.Now()

				rootPieceID := testrand.PieceID()
				pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Index: 0},
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   999999,

						Redundancy: metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Index: 1},
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   999999,

						Redundancy: metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Segments: []metabase.RawSegment{
						{
							StreamID:  obj.StreamID,
							Position:  metabase.SegmentPosition{Index: 0},
							CreatedAt: now,

							RootPieceID:       rootPieceID,
							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							EncryptedSize: 1024,
							PlainSize:     512,
							PlainOffset:   0,

							Redundancy: metabasetest.DefaultRedundancy,

							Pieces: pieces,
						},
						{
							StreamID:  obj.StreamID,
							Position:  metabase.SegmentPosition{Index: 1},
							CreatedAt: now,

							RootPieceID:       rootPieceID,
							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							EncryptedSize: 1024,
							PlainSize:     512,
							PlainOffset:   512,

							Redundancy: metabasetest.DefaultRedundancy,

							Pieces: pieces,
						},
					},
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.CommittedUnversioned,

							SegmentCount:       2,
							FixedSegmentSize:   512,
							TotalPlainSize:     2 * 512,
							TotalEncryptedSize: 2 * 1024,

							Encryption: metabasetest.DefaultEncryption,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("large object over 2 GB", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)
				now := time.Now()

				rootPieceID := testrand.PieceID()
				pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Index: 0},
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: math.MaxInt32,
						PlainSize:     math.MaxInt32,
						Redundancy:    metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Index: 1},
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: math.MaxInt32,
						PlainSize:     math.MaxInt32,
						Redundancy:    metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Segments: []metabase.RawSegment{
						{
							StreamID:  obj.StreamID,
							Position:  metabase.SegmentPosition{Index: 0},
							CreatedAt: now,

							RootPieceID:       rootPieceID,
							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							EncryptedSize: math.MaxInt32,
							PlainSize:     math.MaxInt32,

							Redundancy: metabasetest.DefaultRedundancy,

							Pieces: pieces,
						},
						{
							StreamID:  obj.StreamID,
							Position:  metabase.SegmentPosition{Index: 1},
							CreatedAt: now,

							RootPieceID:       rootPieceID,
							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,

							EncryptedSize: math.MaxInt32,
							PlainSize:     math.MaxInt32,
							PlainOffset:   math.MaxInt32,

							Redundancy: metabasetest.DefaultRedundancy,

							Pieces: pieces,
						},
					},
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.CommittedUnversioned,

							SegmentCount:       2,
							FixedSegmentSize:   math.MaxInt32,
							TotalPlainSize:     2 * math.MaxInt32,
							TotalEncryptedSize: 2 * math.MaxInt32,

							Encryption: metabasetest.DefaultEncryption,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit with encryption", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
					},
				}.Check(ctx, t, db)

				now := time.Now()

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
						Encryption:   storj.EncryptionParameters{},
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "Encryption is missing",
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
						Encryption: storj.EncryptionParameters{
							CipherSuite: storj.EncAESGCM,
						},
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "Encryption.BlockSize is negative or zero",
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
						Encryption: storj.EncryptionParameters{
							CipherSuite: storj.EncAESGCM,
							BlockSize:   -1,
						},
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "Encryption.BlockSize is negative or zero",
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
						Encryption: storj.EncryptionParameters{
							CipherSuite: storj.EncAESGCM,
							BlockSize:   512,
						},
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.CommittedUnversioned,

							SegmentCount: 0,

							Encryption: storj.EncryptionParameters{
								CipherSuite: storj.EncAESGCM,
								BlockSize:   512,
							},
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit with encryption (no override)", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				now := time.Now()

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
						// set different encryption than with BeginObjectExactVersion
						Encryption: storj.EncryptionParameters{
							CipherSuite: storj.EncNull,
							BlockSize:   512,
						},
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.CommittedUnversioned,

							SegmentCount: 0,
							Encryption:   metabasetest.DefaultEncryption,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit with metadata (no overwrite)", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()

				expectedMetadata := testrand.Bytes(memory.KiB)
				expectedMetadataKey := testrand.Bytes(32)
				expectedMetadataNonce := testrand.Nonce().Bytes()

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,

						EncryptedMetadata:             expectedMetadata,
						EncryptedMetadataEncryptedKey: expectedMetadataKey,
						EncryptedMetadataNonce:        expectedMetadataNonce,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.CommittedUnversioned,

							Encryption: metabasetest.DefaultEncryption,

							EncryptedMetadata:             expectedMetadata,
							EncryptedMetadataEncryptedKey: expectedMetadataKey,
							EncryptedMetadataNonce:        expectedMetadataNonce,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit with metadata (overwrite)", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()

				expectedMetadata := testrand.Bytes(memory.KiB)
				expecedMetadataKey := testrand.Bytes(32)
				expecedMetadataNonce := testrand.Nonce().Bytes()

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,

						EncryptedMetadata:             testrand.Bytes(memory.KiB),
						EncryptedMetadataEncryptedKey: testrand.Bytes(32),
						EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					},
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,

						OverrideEncryptedMetadata:     true,
						EncryptedMetadata:             expectedMetadata,
						EncryptedMetadataEncryptedKey: expecedMetadataKey,
						EncryptedMetadataNonce:        expecedMetadataNonce,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.CommittedUnversioned,

							Encryption: metabasetest.DefaultEncryption,

							EncryptedMetadata:             expectedMetadata,
							EncryptedMetadataEncryptedKey: expecedMetadataKey,
							EncryptedMetadataNonce:        expecedMetadataNonce,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit with empty metadata (overwrite)", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,

						EncryptedMetadata:             testrand.Bytes(memory.KiB),
						EncryptedMetadataEncryptedKey: testrand.Bytes(32),
						EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					},
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,

						OverrideEncryptedMetadata:     true,
						EncryptedMetadata:             nil,
						EncryptedMetadataEncryptedKey: nil,
						EncryptedMetadataNonce:        nil,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.CommittedUnversioned,

							Encryption: metabasetest.DefaultEncryption,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit with retention configuration", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()

				retention := metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: now.Add(time.Minute),
				}

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    retention,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.CommittedUnversioned,
							Encryption:   metabasetest.DefaultEncryption,
							Retention:    retention,
						},
					},
				}.Check(ctx, t, db)
			})

			t.Run("commit with retention configuration and expiration", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				future := time.Now().Add(time.Minute)

				object := metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
						Retention: metabase.Retention{
							Mode:        storj.ComplianceMode,
							RetainUntil: future,
						},
						ExpiresAt:           &future,
						TestingBypassVerify: true,
					},
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
					},
					ErrClass: &metabase.Error,
					ErrText:  "object expiration must not be set if retention is set",
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{metabase.RawObject(object)},
				}.Check(ctx, t, db)
			})
		})
	}
}

func TestCommitObjectVersioned(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		obj.Version = metabase.NextVersion
		now := time.Now()
		zombieExpiration := now.Add(24 * time.Hour)

		t.Run("Commit versioned only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			v1 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v1,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 1,
			}.Check(ctx, t, db)
			v1.Version = 1

			v2 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v2,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 2,
			}.Check(ctx, t, db)
			v2.Version = 2

			v3 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v3,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 3,
			}.Check(ctx, t, db)
			v3.Version = 3

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           v1,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v2,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v3,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v1,
					Versioned:    true,
				},
				ExpectVersion: v3.Version + 1,
			}.Check(ctx, t, db)
			v1.Version = v3.Version + 1

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v2,
					Versioned:    true,
				},
				ExpectVersion: v3.Version + 2,
			}.Check(ctx, t, db)
			v2.Version = v1.Version + 1

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v3,
					Versioned:    true,
				},
				ExpectVersion: v3.Version + 3,
			}.Check(ctx, t, db)
			v3.Version = v2.Version + 1

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: v1,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: v2,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: v3,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Commit unversioned then versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			v1 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v1,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 1,
			}.Check(ctx, t, db)
			v1.Version = 1

			v2 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v2,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 2,
			}.Check(ctx, t, db)
			v2.Version = 2

			v3 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v3,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 3,
			}.Check(ctx, t, db)
			v3.Version = 3

			v4 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v4,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 4,
			}.Check(ctx, t, db)
			v4.Version = 4

			// allow having multiple pending objects

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           v1,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v2,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v3,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v4,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v3,
				},
				ExpectVersion: 5,
			}.Check(ctx, t, db)
			v3.Version = 5

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v1,
				},
				ExpectVersion: 6,
			}.Check(ctx, t, db)
			v1.Version = 6

			// The latter commit should overwrite the v3.
			// When pending objects table is enabled, then objects
			// get the version during commit, hence the latest version
			// will be the max.

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: v1,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream:           v2,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v4,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v2,
					Versioned:    true,
				},
				ExpectVersion: 7,
			}.Check(ctx, t, db)
			v2.Version = 7

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v4,
					Versioned:    true,
				},
				ExpectVersion: 8,
			}.Check(ctx, t, db)
			v4.Version = 8

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: v1,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: v2,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: v4,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Commit versioned then unversioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			v1 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v1,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 1,
			}.Check(ctx, t, db)
			v1.Version = 1

			v2 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v2,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 2,
			}.Check(ctx, t, db)
			v2.Version = 2

			v3 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v3,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 3,
			}.Check(ctx, t, db)
			v3.Version = 3

			v4 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v4,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 4,
			}.Check(ctx, t, db)
			v4.Version = 4

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           v1,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v2,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v3,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v4,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v1,
					Versioned:    true,
				},
				ExpectVersion: 5,
			}.Check(ctx, t, db)
			v1.Version = 5

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v3,
					Versioned:    true,
				},
				ExpectVersion: 6,
			}.Check(ctx, t, db)
			v3.Version = 6

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: v1,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream:           v2,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream: v3,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream:           v4,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v2,
				},
				ExpectVersion: 7,
			}.Check(ctx, t, db)
			v2.Version = 7

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v4,
				},
				ExpectVersion: 8,
			}.Check(ctx, t, db)
			v4.Version = 8

			// committing v4 should overwrite the previous unversioned commit (v2),
			// so v2 is not in the result check
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: v1,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: v3,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: v4,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Commit mixed versioned and unversioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			v1 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v1,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 1,
			}.Check(ctx, t, db)
			v1.Version = 1

			v2 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v2,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 2,
			}.Check(ctx, t, db)
			v2.Version = 2

			v3 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v3,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 3,
			}.Check(ctx, t, db)
			v3.Version = 3

			v4 := obj
			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           v4,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &zombieExpiration,
				},
				Version: 4,
			}.Check(ctx, t, db)
			v4.Version = 4

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           v1,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v2,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v3,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v4,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v1,
					Versioned:    true,
				},
				ExpectVersion: 5,
			}.Check(ctx, t, db)
			v1.Version = 5

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: v1,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream:           v2,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v3,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v4,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v2,
				},
				ExpectVersion: 6,
			}.Check(ctx, t, db)
			v2.Version = 6

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: v1,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: v2,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream:           v3,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					{
						ObjectStream:           v4,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v3,
					Versioned:    true,
				},
				ExpectVersion: 7,
			}.Check(ctx, t, db)
			v3.Version = 7

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: v1,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: v2,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: v3,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream:           v4,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: v4,
				},
				ExpectVersion: 8,
			}.Check(ctx, t, db)
			v4.Version = 8

			// committing v4 should overwrite the previous unversioned commit (v2),
			// so v2 is not in the result check
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: v1,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: v3,
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: v4,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Commit large number mixed versioned and unversioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// half the commits are versioned half are unversioned
			numCommits := 1000

			if db.Implementation() == dbutil.Spanner {
				t.Log("TODO(spanner): spanner emulator is too slow for this test, reducing the number to 50")
				numCommits = 50
			}

			objs := make([]*metabase.ObjectStream, numCommits)
			for i := 0; i < numCommits; i++ {
				v := obj
				objs[i] = &v
				metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream:           v,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieExpiration,
					},
					Version: metabase.Version(i + 1),
				}.Check(ctx, t, db)
				v.Version = metabase.Version(i + 1)
			}

			rawObjects := make([]metabase.RawObject, 0, len(objs))
			expectVersion := metabase.Version(numCommits + 1)
			for i := 0; i < len(objs); i++ {
				versioned := i%2 == 0

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: *objs[i],
						Versioned:    versioned,
					},
					ExpectVersion: expectVersion,
				}.Check(ctx, t, db)
				objs[i].Version = expectVersion
				expectVersion++

				if versioned {
					rawObjects = append(rawObjects, metabase.RawObject{
						ObjectStream: *objs[i],
						CreatedAt:    now,
						Status:       metabase.CommittedVersioned,
						Encryption:   metabasetest.DefaultEncryption,
					})
				}
			}

			// all the unversioned commits overwrite previous unversioned commits,
			// so the result should only contain a single unversioned commit.
			rawObjects = append(rawObjects, metabase.RawObject{
				ObjectStream: *objs[len(objs)-1],
				CreatedAt:    now,
				Status:       metabase.CommittedUnversioned,
				Encryption:   metabasetest.DefaultEncryption,
			})
			metabasetest.Verify{
				Objects: rawObjects,
			}.Check(ctx, t, db)
		})
	})
}

func TestCommitObjectWithIncorrectPartSize(t *testing.T) {
	metabasetest.RunWithConfig(t, metabase.Config{
		ApplicationName:  "satellite-test",
		MinPartSize:      5 * memory.MiB,
		MaxNumberOfParts: 1000,
	}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		t.Run("part size less then 5MB", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()
			zombieDeadline := now.Add(24 * time.Hour)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Nonce()

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce[:],

					EncryptedSize: 2 * memory.MiB.Int32(),
					PlainSize:     2 * memory.MiB.Int32(),
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 1, Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce[:],

					EncryptedSize: 2 * memory.MiB.Int32(),
					PlainSize:     2 * memory.MiB.Int32(),
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
				ErrClass: &metabase.ErrFailedPrecondition,
				ErrText:  "size of part number 0 is below minimum threshold, got: 2.0 MiB, min: 5.0 MiB",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 0, Index: 0},
						CreatedAt: now,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: 2 * memory.MiB.Int32(),
						PlainSize:     2 * memory.MiB.Int32(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 1, Index: 0},
						CreatedAt: now,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: 2 * memory.MiB.Int32(),
						PlainSize:     2 * memory.MiB.Int32(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("size validation with part with multiple segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Nonce()

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 1, Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce[:],

					EncryptedSize: memory.MiB.Int32(),
					PlainSize:     memory.MiB.Int32(),
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 1, Index: 1},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce[:],

					EncryptedSize: memory.MiB.Int32(),
					PlainSize:     memory.MiB.Int32(),
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,

						SegmentCount:       2,
						FixedSegmentSize:   -1,
						TotalPlainSize:     2 * memory.MiB.Int64(),
						TotalEncryptedSize: 2 * memory.MiB.Int64(),

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 1, Index: 0},
						CreatedAt: now,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: memory.MiB.Int32(),
						PlainSize:     memory.MiB.Int32(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 1, Index: 1},
						CreatedAt: now,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: memory.MiB.Int32(),
						PlainSize:     memory.MiB.Int32(),
						PlainOffset:   memory.MiB.Int64(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("size validation with multiple parts", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()
			zombieDeadline := now.Add(24 * time.Hour)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Nonce()
			partsSizes := []memory.Size{6 * memory.MiB, 1 * memory.MiB, 1 * memory.MiB}

			for i, size := range partsSizes {
				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: uint32(i + 1), Index: 1},
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: size.Int32(),
						PlainSize:     size.Int32(),
						Redundancy:    metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)
			}

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
				ErrClass: &metabase.ErrFailedPrecondition,
				ErrText:  "size of part number 2 is below minimum threshold, got: 1.0 MiB, min: 5.0 MiB",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 1, Index: 1},
						CreatedAt: now,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: 6 * memory.MiB.Int32(),
						PlainSize:     6 * memory.MiB.Int32(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 2, Index: 1},
						CreatedAt: now,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: memory.MiB.Int32(),
						PlainSize:     memory.MiB.Int32(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 3, Index: 1},
						CreatedAt: now,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: memory.MiB.Int32(),
						PlainSize:     memory.MiB.Int32(),

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestCommitObjectWithIncorrectAmountOfParts(t *testing.T) {
	metabasetest.RunWithConfig(t, metabase.Config{
		ApplicationName:  "satellite-test",
		MinPartSize:      5 * memory.MiB,
		MaxNumberOfParts: 3,
	}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		t.Run("number of parts check", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()
			zombieDeadline := now.Add(24 * time.Hour)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Nonce()

			var segments []metabase.RawSegment

			for i := 1; i < 5; i++ {
				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: uint32(i), Index: 0},
						RootPieceID:  rootPieceID,
						Pieces:       pieces,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce[:],

						EncryptedSize: 6 * memory.MiB.Int32(),
						PlainSize:     6 * memory.MiB.Int32(),
						Redundancy:    metabasetest.DefaultRedundancy,
					},
				}.Check(ctx, t, db)

				segments = append(segments, metabase.RawSegment{
					StreamID:  obj.StreamID,
					Position:  metabase.SegmentPosition{Part: uint32(i), Index: 0},
					CreatedAt: now,

					RootPieceID:       rootPieceID,
					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce[:],

					EncryptedSize: 6 * memory.MiB.Int32(),
					PlainSize:     6 * memory.MiB.Int32(),

					Redundancy: metabasetest.DefaultRedundancy,

					Pieces: pieces,
				})
			}

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
				ErrClass: &metabase.ErrFailedPrecondition,
				ErrText:  "exceeded maximum number of parts: 3",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
				Segments: segments,
			}.Check(ctx, t, db)
		})
	})
}

func TestCommitInlineObject(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		obj.Version = 0

		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("invalid EncryptedMetadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    metabase.DefaultVersion,
						StreamID:   obj.StreamID,
					},
					OverrideEncryptedMetadata: true,
					EncryptedMetadata:         testrand.BytesInt(32),
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set if EncryptedMetadata is set",
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    metabase.DefaultVersion,
						StreamID:   obj.StreamID,
					},
					OverrideEncryptedMetadata:     true,
					EncryptedMetadataEncryptedKey: testrand.BytesInt(32),
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be not set if EncryptedMetadata is not set",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid request", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: obj,
					CommitInlineSegment: metabase.CommitInlineSegment{
						ObjectStream: obj,
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKey missing",
			}.Check(ctx, t, db)

			metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: obj,
					CommitInlineSegment: metabase.CommitInlineSegment{
						ObjectStream: obj,
						InlineData:   []byte{1, 2, 3},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKey missing",
			}.Check(ctx, t, db)

			metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: obj,
					CommitInlineSegment: metabase.CommitInlineSegment{
						ObjectStream: obj,
						InlineData:   []byte{1, 2, 3},

						EncryptedKey: testrand.Bytes(32),
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKeyNonce missing",
			}.Check(ctx, t, db)

			metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: obj,

					CommitInlineSegment: metabase.CommitInlineSegment{
						ObjectStream: obj,
						InlineData:   []byte{1, 2, 3},

						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),

						PlainSize:   512,
						PlainOffset: -1,
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "PlainOffset negative",
			}.Check(ctx, t, db)
		})

		t.Run("commit inline object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)
			inlineData := testrand.Bytes(100)

			object := metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
					CommitInlineSegment: metabase.CommitInlineSegment{
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,
						PlainSize:         512,
						InlineData:        inlineData,
					},
				},
				ExpectVersion: 1,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						CreatedAt: now1,

						RootPieceID:       storj.PieceID{},
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: int32(len(inlineData)),
						PlainOffset:   0,
						PlainSize:     512,
						InlineData:    inlineData,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("overwrite", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// create object to override
			objA := obj

			objA.Version = 123
			metabasetest.CreateObject(ctx, t, db, objA, 2)

			objB := obj
			objB.StreamID = testrand.UUID()

			now1 := time.Now()
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)
			inlineData := testrand.Bytes(100)

			object := metabasetest.CommitInlineObject{
				Opts: metabase.CommitInlineObject{
					ObjectStream: objB,
					Encryption:   metabasetest.DefaultEncryption,
					CommitInlineSegment: metabase.CommitInlineSegment{
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,
						PlainSize:         512,
						InlineData:        inlineData,
					},
				},
				ExpectVersion: objA.Version + 1,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  objB.StreamID,
						CreatedAt: now1,

						RootPieceID:       storj.PieceID{},
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: int32(len(inlineData)),
						PlainOffset:   0,
						PlainSize:     512,
						InlineData:    inlineData,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit inline object versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()
			obj := obj

			expectedObjects := []metabase.RawObject{}
			expectedSegments := []metabase.RawSegment{}
			for i := 0; i < 3; i++ {
				encryptedKey := testrand.Bytes(32)
				encryptedKeyNonce := testrand.Bytes(32)
				inlineData := testrand.Bytes(100)

				obj.StreamID = testrand.UUID()
				expectedObjects = append(expectedObjects, metabase.RawObject(metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
						CommitInlineSegment: metabase.CommitInlineSegment{
							EncryptedKey:      encryptedKey,
							EncryptedKeyNonce: encryptedKeyNonce,
							PlainSize:         512,
							InlineData:        inlineData,
						},

						Versioned: true,
					},
					ExpectVersion: metabase.Version(i + 1),
				}.Check(ctx, t, db)))

				expectedSegments = append(expectedSegments, metabase.RawSegment{
					StreamID:  obj.StreamID,
					CreatedAt: now1,

					RootPieceID:       storj.PieceID{},
					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: int32(len(inlineData)),
					PlainOffset:   0,
					PlainSize:     512,
					InlineData:    inlineData,
				})
			}

			metabasetest.Verify{
				Objects:  expectedObjects,
				Segments: expectedSegments,
			}.Check(ctx, t, db)
		})

		t.Run("retention", func(t *testing.T) {
			commitInlineSeg := metabase.CommitInlineSegment{
				EncryptedKey:      testrand.Bytes(32),
				EncryptedKeyNonce: testrand.Bytes(32),
				PlainSize:         512,
				InlineData:        testrand.Bytes(100),
			}

			t.Run("success", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()

				retention := metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: now.Add(time.Minute),
				}

				metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream:        obj,
						Encryption:          metabasetest.DefaultEncryption,
						CommitInlineSegment: commitInlineSeg,
						Retention:           retention,
					},
					ExpectVersion: 1,
				}.Check(ctx, t, db)

				expectedObjStream := obj
				expectedObjStream.Version = 1
				metabasetest.Verify{
					Objects: []metabase.RawObject{{
						ObjectStream: expectedObjStream,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    retention,

						SegmentCount:       1,
						TotalPlainSize:     int64(commitInlineSeg.PlainSize),
						TotalEncryptedSize: int64(len(commitInlineSeg.InlineData)),
					}},
					Segments: []metabase.RawSegment{{
						StreamID:          obj.StreamID,
						CreatedAt:         now,
						EncryptedKeyNonce: commitInlineSeg.EncryptedKeyNonce,
						EncryptedKey:      commitInlineSeg.EncryptedKey,
						EncryptedSize:     int32(len(commitInlineSeg.InlineData)),
						PlainSize:         commitInlineSeg.PlainSize,
						InlineData:        commitInlineSeg.InlineData,
					}},
				}.Check(ctx, t, db)
			})

			t.Run("invalid retention configuration", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				check := func(mode storj.RetentionMode, retainUntil time.Time, errText string) {
					metabasetest.CommitInlineObject{
						Opts: metabase.CommitInlineObject{
							ObjectStream:        obj,
							Encryption:          metabasetest.DefaultEncryption,
							CommitInlineSegment: commitInlineSeg,
							Retention: metabase.Retention{
								Mode:        mode,
								RetainUntil: retainUntil,
							},
						},
						ErrClass: &metabase.ErrInvalidRequest,
						ErrText:  errText,
					}.Check(ctx, t, db)
				}

				now := time.Now()

				check(storj.ComplianceMode, time.Time{}, "retention period expiration must be set if retention mode is set")
				check(storj.NoRetention, now.Add(time.Minute), "retention period expiration must not be set if retention mode is not set")
				check(storj.RetentionMode(2), now.Add(time.Minute), "retention mode must be 0 (none) or 1 (compliance), but it was 2")

				metabasetest.Verify{}.Check(ctx, t, db)
			})

			t.Run("retention configuration with TTL", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()
				expires := now.Add(time.Minute)

				metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream:        obj,
						Encryption:          metabasetest.DefaultEncryption,
						CommitInlineSegment: commitInlineSeg,
						Retention: metabase.Retention{
							Mode:        storj.ComplianceMode,
							RetainUntil: now.Add(time.Minute),
						},
						ExpiresAt: &expires,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "ExpiresAt must not be set if Retention is set",
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		})
	})
}

func TestOverwriteLockedObject(t *testing.T) {
	// This tests a case where an object is committed to an unversioned bucket, but
	// an object version with an active Object Lock configuration is already present
	// in its place. We don't expect any unversioned objects to have Object Lock
	// configurations, but we must ensure that we handle them properly in case we
	// introduce a bug that allows them to exist.
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		objStream := metabasetest.RandObjectStream()

		createLockedObject := func(t *testing.T, objStream metabase.ObjectStream, retainUntil time.Time) (metabase.Object, []metabase.Segment) {
			return metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					Retention: metabase.Retention{
						Mode:        storj.ComplianceMode,
						RetainUntil: retainUntil,
					},
				},
			}.Run(ctx, t, db, objStream, 1)
		}

		t.Run("CommitObject", func(t *testing.T) {
			t.Run("Active retention period", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				lockedObj, lockedSegs := createLockedObject(t, objStream, time.Now().Add(time.Hour))

				beginObjStream := objStream
				beginObjStream.Version = metabase.NextVersion
				obj := metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream: beginObjStream,
						Encryption:   metabasetest.DefaultEncryption,
					},
					Version: lockedObj.Version + 1,
				}.Check(ctx, t, db)

				metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream:  obj.ObjectStream,
						UseObjectLock: true,
					},
					ErrClass: &metabase.ErrObjectLock,
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects:  []metabase.RawObject{metabase.RawObject(lockedObj), metabase.RawObject(obj)},
					Segments: []metabase.RawSegment{metabase.RawSegment(lockedSegs[0])},
				}.Check(ctx, t, db)
			})

			t.Run("Active retention period - UseObjectLock disabled", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				lockedObj, _ := createLockedObject(t, objStream, time.Now().Add(time.Hour))

				beginObjStream := objStream
				beginObjStream.Version = metabase.NextVersion
				obj := metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream: beginObjStream,
						Encryption:   metabasetest.DefaultEncryption,
					},
					Version: lockedObj.Version + 1,
				}.Check(ctx, t, db)

				obj = metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream:  obj.ObjectStream,
						UseObjectLock: false,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{metabase.RawObject(obj)},
				}.Check(ctx, t, db)
			})

			t.Run("Expired retention period", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				lockedObj, _ := createLockedObject(t, objStream, time.Now().Add(-time.Minute))

				beginObjStream := objStream
				beginObjStream.Version = metabase.NextVersion
				obj := metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream: beginObjStream,
						Encryption:   metabasetest.DefaultEncryption,
					},
					Version: lockedObj.Version + 1,
				}.Check(ctx, t, db)

				obj = metabasetest.CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream:  obj.ObjectStream,
						UseObjectLock: true,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: []metabase.RawObject{metabase.RawObject(obj)},
				}.Check(ctx, t, db)
			})
		})

		t.Run("CommitInlineObject", func(t *testing.T) {
			getExpectedInlineSegment := func(obj metabase.Object, commitInlineSeg metabase.CommitInlineSegment) metabase.RawSegment {
				return metabase.RawSegment{
					StreamID:          obj.StreamID,
					CreatedAt:         obj.CreatedAt,
					EncryptedKeyNonce: commitInlineSeg.EncryptedKeyNonce,
					EncryptedKey:      commitInlineSeg.EncryptedKey,
					EncryptedSize:     int32(len(commitInlineSeg.InlineData)),
					PlainSize:         commitInlineSeg.PlainSize,
					InlineData:        commitInlineSeg.InlineData,
				}
			}

			commitInlineSeg := metabase.CommitInlineSegment{
				EncryptedKey:      testrand.Bytes(32),
				EncryptedKeyNonce: testrand.Bytes(32),
				PlainSize:         512,
				InlineData:        testrand.Bytes(100),
			}

			t.Run("Active retention period", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				lockedObj, lockedSegs := createLockedObject(t, objStream, time.Now().Add(time.Hour))

				metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream:        objStream,
						Encryption:          metabasetest.DefaultEncryption,
						CommitInlineSegment: commitInlineSeg,
						UseObjectLock:       true,
					},
					ErrClass: &metabase.ErrObjectLock,
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects:  []metabase.RawObject{metabase.RawObject(lockedObj)},
					Segments: []metabase.RawSegment{metabase.RawSegment(lockedSegs[0])},
				}.Check(ctx, t, db)
			})

			t.Run("Active retention period - UseObjectLock disabled", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()
				lockedObj, _ := createLockedObject(t, objStream, now.Add(time.Hour))

				inlineObj := metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream:        objStream,
						Encryption:          metabasetest.DefaultEncryption,
						CommitInlineSegment: commitInlineSeg,
					},
					ExpectVersion: lockedObj.Version + 1,
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects:  []metabase.RawObject{metabase.RawObject(inlineObj)},
					Segments: []metabase.RawSegment{getExpectedInlineSegment(inlineObj, commitInlineSeg)},
				}.Check(ctx, t, db)
			})

			t.Run("Expired retention period", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				objStream := metabasetest.RandObjectStream()
				lockedObj, _ := createLockedObject(t, objStream, time.Now().Add(-time.Minute))

				inlineObj := metabasetest.CommitInlineObject{
					Opts: metabase.CommitInlineObject{
						ObjectStream:        objStream,
						Encryption:          metabasetest.DefaultEncryption,
						CommitInlineSegment: commitInlineSeg,
						UseObjectLock:       true,
					},
					ExpectVersion: lockedObj.Version + 1,
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects:  []metabase.RawObject{metabase.RawObject(inlineObj)},
					Segments: []metabase.RawSegment{getExpectedInlineSegment(inlineObj, commitInlineSeg)},
				}.Check(ctx, t, db)
			})
		})
	})
}
