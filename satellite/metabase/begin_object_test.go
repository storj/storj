// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"fmt"
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
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

			for i, scenario := range metabasetest.InvalidEncryptedUserDataScenarios() {
				t.Log(i)

				stream := objectStream
				stream.ObjectKey = metabase.ObjectKey(fmt.Sprint(i))
				opts := metabase.BeginObjectNextVersion{
					ObjectStream:      stream,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: scenario.EncryptedUserData,
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

			futureTime := time.Now().Add(10 * 24 * time.Hour)

			objectStream.Version = metabase.NextVersion

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
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    2,
						StreamID:   obj.StreamID,
					},
				},
			}.Check(ctx, t, db)

			// currently CommitObject always deletes previous versions so only version 2 left
			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("newer committed version exists", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

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

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    1,
						StreamID:   obj.StreamID,
					},
				},
				ExpectVersion: 2,
			}.Check(ctx, t, db)

			// currently CommitObject always deletes previous versions so only version 1 left
			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("begin object next version with metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objectStream.Version = metabase.NextVersion
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

			objectStream.Version = metabase.NextVersion

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

			for i, scenario := range metabasetest.InvalidEncryptedUserDataScenarios() {
				t.Log(i)

				stream := objectStream
				stream.Version = 5
				stream.ObjectKey = metabase.ObjectKey(fmt.Sprint(i))

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream:      stream,
						EncryptedUserData: scenario.EncryptedUserData,
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

			objectStream.Version = 5

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

			objectStream.Version = 5

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

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream,
				},
			}.Check(ctx, t, db)

			// currently CommitObject always deletes previous versions so only version 3 left
			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
			}.Check(ctx, t, db)
		})

		t.Run("Newer committed version exists", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

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

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream,
				},
				ExpectVersion: 300,
			}.Check(ctx, t, db)

			// currently CommitObject always deletes previous versions so only version 1 left
			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
			}.Check(ctx, t, db)
		})

		t.Run("begin object exact version with metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objectStream.Version = 100

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

			objectStream.Version = 100

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
	})
}
