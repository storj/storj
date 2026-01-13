// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil/spannerutil"
)

func TestBeginObjectNextVersion(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		objectStream := metabasetest.RandObjectStream()
		objectStream.Version = metabase.NextVersion

		for _, test := range metabasetest.InvalidObjectStreams(objectStream) {
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

			for i, scenario := range metabasetest.InvalidEncryptedUserDataScenariosForBegin() {
				userData := scenario.EncryptedUserData

				t.Log(i)

				stream := objectStream
				stream.ObjectKey = metabase.ObjectKey(fmt.Sprint(i))
				opts := metabase.BeginObjectNextVersion{
					ObjectStream:      stream,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				}

				metabasetest.BeginObjectNextVersion{
					Opts:     opts,
					Version:  -1,
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  scenario.ErrText,
				}.Check(ctx, t, db)
			}

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("disallow exact version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := objectStream
			stream.Version = 5

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: stream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version:  -1,
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version should be metabase.NextVersion",
			}.Check(ctx, t, db)
		})

		t.Run("NextVersion", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			futureTime := time.Now().Add(10 * 24 * time.Hour)

			object1 := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			object2 := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:           objectStream,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &futureTime,
				},
				Version: 2,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object1),
					metabase.RawObject(object2),
				},
			}.Check(ctx, t, db)
		})

		// TODO: expires at date
		// TODO: zombie deletion deadline

		t.Run("Retention", func(t *testing.T) {
			t.Run("Success", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()

				retention := metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: now.Add(time.Minute),
				}

				object := metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream: objectStream,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    retention,
					},
					Version: 1,
				}.Check(ctx, t, db)

				metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
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
				check(storj.GovernanceMode, time.Time{}, "retention period expiration must be set if retention mode is set")
				check(storj.NoRetention, now.Add(time.Minute), "retention period expiration must not be set if retention mode is not set")
				check(storj.RetentionMode(3), now.Add(time.Minute), "invalid retention mode 3")

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

		t.Run("Legal Hold", func(t *testing.T) {
			t.Run("Success", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()

				retention := metabase.Retention{
					Mode:        storj.GovernanceMode,
					RetainUntil: now.Add(time.Minute),
				}

				object := metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream: objectStream,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    retention,
						LegalHold:    true,
					},
					Version: 1,
				}.Check(ctx, t, db)

				metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
			})

			t.Run("With TTL", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()
				expires := now.Add(time.Minute)

				metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream: objectStream,
						Encryption:   metabasetest.DefaultEncryption,
						LegalHold:    true,
						ExpiresAt:    &expires,
					},
					Version:  1,
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "ExpiresAt must not be set if LegalHold is set",
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		})

		t.Run("older committed version exists", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

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
						ProjectID:  objectStream.ProjectID,
						BucketName: objectStream.BucketName,
						ObjectKey:  objectStream.ObjectKey,
						Version:    1,
						StreamID:   objectStream.StreamID,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 2,
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  objectStream.ProjectID,
						BucketName: objectStream.BucketName,
						ObjectKey:  objectStream.ObjectKey,
						Version:    2,
						StreamID:   objectStream.StreamID,
					},
				},
			}.Check(ctx, t, db)

			// currently CommitObject always deletes previous versions so only version 2 left
			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("newer committed version exists", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

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
						ProjectID:  objectStream.ProjectID,
						BucketName: objectStream.BucketName,
						ObjectKey:  objectStream.ObjectKey,
						Version:    2,
						StreamID:   objectStream.StreamID,
					},
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  objectStream.ProjectID,
						BucketName: objectStream.BucketName,
						ObjectKey:  objectStream.ObjectKey,
						Version:    1,
						StreamID:   objectStream.StreamID,
					},
				},
				ExpectVersion: 2,
			}.Check(ctx, t, db)

			// currently CommitObject always deletes previous versions so only version 1 left
			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("begin object next version with metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			userData := metabasetest.RandEncryptedUserDataWithoutETag()

			object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:      objectStream,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("begin object next version with metadata+etag", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			userData := metabasetest.RandEncryptedUserData()

			object := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:      objectStream,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("Checksum", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			userData := metabasetest.RandEncryptedUserDataWithChecksum()

			object1 := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:      objectStream,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				},
				Version: 1,
			}.Check(ctx, t, db)

			// Ensure that a pending object can be created with checksum options that lack an encrypted checksum.
			userData.Checksum.EncryptedValue = nil
			objectStream2 := metabasetest.RandObjectStream()
			objectStream2.Version = metabase.NextVersion

			object2 := metabasetest.BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream:      objectStream2,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object1, object2)}.Check(ctx, t, db)
		})
	})
}

func TestBeginObjectExactVersion(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		objectStream := metabasetest.RandObjectStream()

		for _, test := range metabasetest.InvalidObjectStreams(objectStream) {
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

		t.Run("invalid EncryptedMetadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			for i, scenario := range metabasetest.InvalidEncryptedUserDataScenariosForBegin() {
				userData := scenario.EncryptedUserData

				t.Log(i)

				stream := objectStream
				stream.Version = 5
				stream.ObjectKey = metabase.ObjectKey(fmt.Sprint(i))

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream:      stream,
						EncryptedUserData: userData,
						Encryption:        metabasetest.DefaultEncryption,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  scenario.ErrText,
				}.Check(ctx, t, db)
			}

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("disallow NextVersion", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := objectStream
			stream.Version = metabase.NextVersion

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: stream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version should not be metabase.NextVersion",
			}.Check(ctx, t, db)
		})

		t.Run("Specific version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("Duplicate pending version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
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

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("Duplicate committed version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()

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
						ObjectStream: objectStream,
						CreatedAt:    now1,
						Status:       metabase.CommittedUnversioned,

						Encryption: metabasetest.DefaultEncryption,
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

				retention := metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: now.Add(time.Minute),
				}

				object := metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: objectStream,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    retention,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
			})

			t.Run("Invalid", func(t *testing.T) {
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
				check(storj.RetentionMode(3), now.Add(time.Minute), "invalid retention mode 3")

				metabasetest.Verify{}.Check(ctx, t, db)
			})

			t.Run("With TTL", func(t *testing.T) {
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

		t.Run("Legal hold", func(t *testing.T) {
			t.Run("Success", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()

				retention := metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: now.Add(time.Minute),
				}

				object := metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: objectStream,
						Encryption:   metabasetest.DefaultEncryption,
						LegalHold:    true,
						// An object's legal hold status and retention mode are stored as a
						// single value in the database. A retention period is provided here
						// to test that these properties are properly encoded.
						Retention: retention,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
			})

			t.Run("With TTL", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				now := time.Now()
				expires := now.Add(time.Minute)

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: objectStream,
						Encryption:   metabasetest.DefaultEncryption,
						LegalHold:    true,
						ExpiresAt:    &expires,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "ExpiresAt must not be set if LegalHold is set",
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		})

		t.Run("Older committed version exists", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

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

			objectStream2 := objectStream
			objectStream2.Version++

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream2,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream2,
				},
			}.Check(ctx, t, db)

			// currently CommitObject always deletes previous versions so only version 3 left
			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
			}.Check(ctx, t, db)
		})

		t.Run("Newer committed version exists", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

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

			objectStream2 := objectStream
			objectStream2.Version--

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream2,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream2,
				},
				ExpectVersion: objectStream.Version,
			}.Check(ctx, t, db)

			// currently CommitObject always deletes previous versions so only version 1 left
			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
			}.Check(ctx, t, db)
		})

		t.Run("begin object exact version with metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			userData := metabasetest.RandEncryptedUserDataWithoutETag()

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:      objectStream,
					EncryptedUserData: userData,
					Encryption:        metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("begin object exact version with metadata+etag", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			userData := metabasetest.RandEncryptedUserData()

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:      objectStream,
					EncryptedUserData: userData,
					Encryption:        metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("Checksum", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			userData := metabasetest.RandEncryptedUserDataWithChecksum()

			object1 := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:      objectStream,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				},
			}.Check(ctx, t, db)

			// Ensure that a pending object can be created with checksum options that lack an encrypted checksum.
			userData.Checksum.EncryptedValue = nil

			object2 := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:      metabasetest.RandObjectStream(),
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object1, object2)}.Check(ctx, t, db)
		})
	})
}

func TestBeginObject_Encoding(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// The purpose of this test is to ensure that we represent the zero values of certain
		// object properties as NULL in the database.

		// RetainUntil is included here to ensure that we encode it properly.
		// Usually, optional timestamps are represented as *time.Time and encoded in the database
		// as NULL only when they are nil. However, RetainUntil is time.Time, so we implement
		// a custom encoder that encodes it as NULL when it is zero (time.Time{}).
		type presentValues struct {
			retentionMode bool
			retainUntil   bool
			checksum      bool
		}

		getValuePresence := func(t *testing.T, objStream metabase.ObjectStream) presentValues {
			query := `
				SELECT
					retention_mode IS NOT NULL,
					retain_until   IS NOT NULL,
					checksum       IS NOT NULL
				FROM objects
				WHERE (project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)`

			args := map[string]any{
				"project_id":  objStream.ProjectID,
				"bucket_name": objStream.BucketName,
				"object_key":  objStream.ObjectKey,
				"version":     objStream.Version,
			}

			adapter := db.ChooseAdapter(uuid.UUID{})

			var isPresent presentValues

			switch ad := adapter.(type) {
			case *metabase.PostgresAdapter:
				row := ad.UnderlyingDB().QueryRowContext(ctx, query, pgx.StrictNamedArgs(args))
				require.NoError(t, row.Scan(
					&isPresent.retentionMode,
					&isPresent.retainUntil,
					&isPresent.checksum,
				))
			case *metabase.CockroachAdapter:
				row := ad.PostgresAdapter.UnderlyingDB().QueryRowContext(ctx, query, pgx.StrictNamedArgs(args))
				require.NoError(t, row.Scan(
					&isPresent.retentionMode,
					&isPresent.retainUntil,
					&isPresent.checksum,
				))
			case *metabase.SpannerAdapter:
				var err error
				isPresent, err = spannerutil.CollectRow(
					ad.UnderlyingDB().Single().QueryWithOptions(ctx, spanner.Statement{
						SQL:    query,
						Params: args,
					}, spanner.QueryOptions{}),
					func(row *spanner.Row, item *presentValues) error {
						return errs.Wrap(row.Columns(
							&item.retentionMode,
							&item.retainUntil,
							&item.checksum,
						))
					},
				)
				require.NoError(t, err)
			default:
				t.Skipf("unknown adapter type %T", adapter)
			}

			return isPresent
		}

		test := func(t *testing.T, apply func(*testing.T, metabase.Retention, metabase.EncryptedUserData) metabase.ObjectStream) {
			// Note: RandEncryptedUserData returns a set of user data with unset checksum properties.
			userData := metabasetest.RandEncryptedUserData()

			objStream := apply(t, metabase.Retention{
				Mode:        storj.NoRetention,
				RetainUntil: time.Time{},
			}, userData)
			isPresent := getValuePresence(t, objStream)
			require.False(t, isPresent.retentionMode)
			require.False(t, isPresent.retainUntil)
			require.False(t, isPresent.checksum)
		}

		t.Run("Next version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			test(t, func(t *testing.T, retention metabase.Retention, userData metabase.EncryptedUserData) metabase.ObjectStream {
				objStream := metabasetest.RandObjectStream()
				objStream.Version = metabase.NextVersion

				object := metabasetest.BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream:      objStream,
						Encryption:        metabasetest.DefaultEncryption,
						Retention:         retention,
						EncryptedUserData: userData,
					},
					Version: 1,
				}.Check(ctx, t, db)
				return object.ObjectStream
			})
		})

		t.Run("Exact version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			test(t, func(t *testing.T, retention metabase.Retention, userData metabase.EncryptedUserData) metabase.ObjectStream {
				object := metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream:      metabasetest.RandObjectStream(),
						Encryption:        metabasetest.DefaultEncryption,
						Retention:         retention,
						EncryptedUserData: userData,
					},
				}.Check(ctx, t, db)
				return object.ObjectStream
			})
		})
	})
}
