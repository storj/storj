// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

// transitionWritesStream builds an object stream for the given project, key, and version.
func transitionWritesStream(projectID uuid.UUID, key string, version metabase.Version) metabase.ObjectStream {
	return metabase.ObjectStream{
		ProjectID:  projectID,
		BucketName: "bucket",
		ObjectKey:  metabase.ObjectKey(key),
		Version:    version,
		StreamID:   testrand.UUID(),
	}
}

// transitionWritesSeedRaw inserts the given committed raw object directly into the
// given backend, bypassing transition routing.
func transitionWritesSeedRaw(ctx *testcontext.Context, t *testing.T, adapter metabase.Adapter, raw metabase.RawObject) {
	if raw.Status == 0 {
		raw.Status = metabase.CommittedUnversioned
	}
	if raw.Encryption == (storj.EncryptionParameters{}) {
		raw.Encryption = metabasetest.DefaultEncryption
	}
	require.NoError(t, adapter.TestingBatchInsertObjects(ctx, []metabase.RawObject{raw}))
}

// transitionWritesObjectIn reports whether the backend holds a committed object at
// the given stream's location/version, by reading from the backend directly.
func transitionWritesObjectIn(ctx *testcontext.Context, t *testing.T, adapter metabase.Adapter, stream metabase.ObjectStream) bool {
	objects, err := adapter.TestingGetAllObjects(ctx)
	require.NoError(t, err)
	return transitionContainsStream(objects, stream.StreamID)
}

// TestTransitionWrites_BeginObjectExactVersion verifies that pending objects created
// through BeginObjectExactVersion are co-located with the backend that already owns
// the location, and otherwise land in primary.
func TestTransitionWrites_BeginObjectExactVersion(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		// N: brand-new key -> pending created in primary.
		newStream := transitionWritesStream(projectID, "new-key", 1)
		_, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
			ObjectStream: newStream,
			Encryption:   metabasetest.DefaultEncryption,
		})
		require.NoError(t, err)
		require.True(t, transitionWritesObjectIn(ctx, t, primary, newStream))
		require.False(t, transitionWritesObjectIn(ctx, t, secondary, newStream))

		// S: existing committed object lives in secondary -> pending co-located in secondary.
		transitionSeedCommitted(ctx, t, secondary, projectID, "only-secondary")
		secondaryStream := transitionWritesStream(projectID, "only-secondary", 2)
		_, err = db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
			ObjectStream: secondaryStream,
			Encryption:   metabasetest.DefaultEncryption,
		})
		require.NoError(t, err)
		require.True(t, transitionWritesObjectIn(ctx, t, secondary, secondaryStream))
		require.False(t, transitionWritesObjectIn(ctx, t, primary, secondaryStream))

		// P: existing committed object lives in primary -> pending stays in primary.
		transitionSeedCommitted(ctx, t, primary, projectID, "only-primary")
		primaryStream := transitionWritesStream(projectID, "only-primary", 2)
		_, err = db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
			ObjectStream: primaryStream,
			Encryption:   metabasetest.DefaultEncryption,
		})
		require.NoError(t, err)
		require.True(t, transitionWritesObjectIn(ctx, t, primary, primaryStream))
		require.False(t, transitionWritesObjectIn(ctx, t, secondary, primaryStream))

		// B: object exists in both -> primary takes precedence.
		bothStream := transitionSeedCommitted(ctx, t, primary, projectID, "in-both")
		transitionWritesSeedRaw(ctx, t, secondary, metabase.RawObject{ObjectStream: bothStream})
		beginBoth := transitionWritesStream(projectID, "in-both", 2)
		_, err = db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
			ObjectStream: beginBoth,
			Encryption:   metabasetest.DefaultEncryption,
		})
		require.NoError(t, err)
		require.True(t, transitionWritesObjectIn(ctx, t, primary, beginBoth))
		require.False(t, transitionWritesObjectIn(ctx, t, secondary, beginBoth))
	})
}

// TestTransitionWrites_BeginObjectNextVersion verifies the same co-location matrix
// for BeginObjectNextVersion.
func TestTransitionWrites_BeginObjectNextVersion(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		beginNext := func(key string) metabase.Object {
			stream := transitionWritesStream(projectID, key, metabase.NextVersion)
			obj, err := db.BeginObjectNextVersion(ctx, metabase.BeginObjectNextVersion{
				ObjectStream: stream,
				Encryption:   metabasetest.DefaultEncryption,
			})
			require.NoError(t, err)
			return obj
		}

		// N: brand-new key -> primary.
		newObj := beginNext("new-key")
		require.True(t, transitionWritesObjectIn(ctx, t, primary, newObj.ObjectStream))
		require.False(t, transitionWritesObjectIn(ctx, t, secondary, newObj.ObjectStream))

		// S: existing in secondary -> secondary.
		transitionSeedCommitted(ctx, t, secondary, projectID, "only-secondary")
		secondaryObj := beginNext("only-secondary")
		require.True(t, transitionWritesObjectIn(ctx, t, secondary, secondaryObj.ObjectStream))
		require.False(t, transitionWritesObjectIn(ctx, t, primary, secondaryObj.ObjectStream))

		// P: existing in primary -> primary.
		transitionSeedCommitted(ctx, t, primary, projectID, "only-primary")
		primaryObj := beginNext("only-primary")
		require.True(t, transitionWritesObjectIn(ctx, t, primary, primaryObj.ObjectStream))
		require.False(t, transitionWritesObjectIn(ctx, t, secondary, primaryObj.ObjectStream))

		// B: exists in both -> primary.
		bothStream := transitionSeedCommitted(ctx, t, primary, projectID, "in-both")
		transitionWritesSeedRaw(ctx, t, secondary, metabase.RawObject{ObjectStream: bothStream})
		bothObj := beginNext("in-both")
		require.True(t, transitionWritesObjectIn(ctx, t, primary, bothObj.ObjectStream))
		require.False(t, transitionWritesObjectIn(ctx, t, secondary, bothObj.ObjectStream))
	})
}

// TestTransitionWrites_CommitObject verifies the full begin->commit lifecycle lands
// the committed object in the backend where Begin placed its pending row.
func TestTransitionWrites_CommitObject(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		// New key: begin lands in primary, commit must land in primary.
		newStream := transitionWritesStream(projectID, "new-key", 1)
		_, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
			ObjectStream: newStream,
			Encryption:   metabasetest.DefaultEncryption,
		})
		require.NoError(t, err)
		_, err = db.CommitObject(ctx, metabase.CommitObject{ObjectStream: newStream})
		require.NoError(t, err)

		obj, err := primary.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
			ObjectLocation: newStream.Location(),
			Version:        newStream.Version,
		})
		require.NoError(t, err)
		require.Equal(t, metabase.CommittedUnversioned, obj.Status)
		_, err = secondary.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
			ObjectLocation: newStream.Location(),
			Version:        newStream.Version,
		})
		require.True(t, metabase.ErrObjectNotFound.Has(err))

		// Existing-secondary location: begin co-locates in secondary, commit falls
		// back to secondary.
		transitionSeedCommitted(ctx, t, secondary, projectID, "only-secondary")
		secondaryStream := transitionWritesStream(projectID, "only-secondary", 2)
		_, err = db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
			ObjectStream: secondaryStream,
			Encryption:   metabasetest.DefaultEncryption,
		})
		require.NoError(t, err)
		_, err = db.CommitObject(ctx, metabase.CommitObject{ObjectStream: secondaryStream})
		require.NoError(t, err)

		require.True(t, transitionWritesObjectIn(ctx, t, secondary, secondaryStream))
		require.False(t, transitionWritesObjectIn(ctx, t, primary, secondaryStream))
	})
}

// TestTransitionWrites_CommitInlineObject verifies that a single-call inline object
// lands in primary for a new key and in the owning backend for a co-located key.
func TestTransitionWrites_CommitInlineObject(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		commitInline := func(stream metabase.ObjectStream) {
			_, err := db.CommitInlineObject(ctx, metabase.CommitInlineObject{
				ObjectStream: stream,
				Encryption:   metabasetest.DefaultEncryption,
				CommitInlineSegment: metabase.CommitInlineSegment{
					ObjectStream:      stream,
					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(24),
					PlainSize:         4,
					InlineData:        []byte{1, 2, 3, 4},
				},
			})
			require.NoError(t, err)
		}

		// New key -> primary.
		newStream := transitionWritesStream(projectID, "new-inline", 1)
		commitInline(newStream)
		require.True(t, transitionWritesObjectIn(ctx, t, primary, newStream))
		require.False(t, transitionWritesObjectIn(ctx, t, secondary, newStream))

		// Co-located key (owned by secondary) -> secondary via write-fallback.
		transitionSeedCommitted(ctx, t, secondary, projectID, "secondary-inline")
		secondaryStream := transitionWritesStream(projectID, "secondary-inline", 2)
		commitInline(secondaryStream)
		require.True(t, transitionWritesObjectIn(ctx, t, secondary, secondaryStream))
		require.False(t, transitionWritesObjectIn(ctx, t, primary, secondaryStream))
	})
}

// TestTransitionWrites_CommitSegment verifies that segment commits route to the
// backend holding the pending object: primary when Begin placed it there, secondary
// when fallback applies.
func TestTransitionWrites_CommitSegment(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		commitSegment := func(stream metabase.ObjectStream) {
			err := db.BeginSegment(ctx, metabase.BeginSegment{
				ObjectStream: stream,
				Position:     metabase.SegmentPosition{Index: 0},
				RootPieceID:  storj.PieceID{1},
				Pieces: metabase.Pieces{{
					Number:      1,
					StorageNode: testrand.NodeID(),
				}},
			})
			require.NoError(t, err)
			err = db.CommitSegment(ctx, metabase.CommitSegment{
				ObjectStream:      stream,
				Position:          metabase.SegmentPosition{Index: 0},
				RootPieceID:       storj.PieceID{1},
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				EncryptedKey:      testrand.Bytes(32),
				EncryptedKeyNonce: testrand.Bytes(24),
				EncryptedSize:     1024,
				PlainSize:         512,
				PlainOffset:       0,
				Redundancy:        metabasetest.DefaultRedundancy,
			})
			require.NoError(t, err)
		}

		// Pending in primary (new key).
		primaryStream := transitionWritesStream(projectID, "new-key", 1)
		_, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
			ObjectStream: primaryStream,
			Encryption:   metabasetest.DefaultEncryption,
		})
		require.NoError(t, err)
		commitSegment(primaryStream)

		primarySegments, err := primary.TestingGetAllSegments(ctx, metabase.NewNodeAliasCache(primary, true))
		require.NoError(t, err)
		require.Len(t, primarySegments, 1)
		require.Equal(t, primaryStream.StreamID, primarySegments[0].StreamID)

		// Pending in secondary (co-located with an existing secondary object).
		transitionSeedCommitted(ctx, t, secondary, projectID, "only-secondary")
		secondaryStream := transitionWritesStream(projectID, "only-secondary", 2)
		_, err = db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
			ObjectStream: secondaryStream,
			Encryption:   metabasetest.DefaultEncryption,
		})
		require.NoError(t, err)
		commitSegment(secondaryStream)

		// Node aliases live on the shared backend (adapter 0 / primary), so resolve
		// the secondary's segment pieces with a primary-backed cache.
		secondarySegments, err := secondary.TestingGetAllSegments(ctx, metabase.NewNodeAliasCache(primary, true))
		require.NoError(t, err)
		require.Len(t, secondarySegments, 1)
		require.Equal(t, secondaryStream.StreamID, secondarySegments[0].StreamID)
	})
}

// TestTransitionWrites_CommitInlineSegment verifies inline segment commits route to
// the backend holding the pending object.
func TestTransitionWrites_CommitInlineSegment(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		commitInlineSegment := func(stream metabase.ObjectStream) {
			err := db.CommitInlineSegment(ctx, metabase.CommitInlineSegment{
				ObjectStream:      stream,
				Position:          metabase.SegmentPosition{Index: 0},
				EncryptedKey:      testrand.Bytes(32),
				EncryptedKeyNonce: testrand.Bytes(24),
				PlainSize:         4,
				InlineData:        []byte{1, 2, 3, 4},
			})
			require.NoError(t, err)
		}

		// Pending in primary (new key).
		primaryStream := transitionWritesStream(projectID, "new-key", 1)
		_, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
			ObjectStream: primaryStream,
			Encryption:   metabasetest.DefaultEncryption,
		})
		require.NoError(t, err)
		commitInlineSegment(primaryStream)

		primarySegments, err := primary.TestingGetAllSegments(ctx, metabase.NewNodeAliasCache(primary, true))
		require.NoError(t, err)
		require.Len(t, primarySegments, 1)
		require.Equal(t, primaryStream.StreamID, primarySegments[0].StreamID)

		// Pending in secondary (co-located).
		transitionSeedCommitted(ctx, t, secondary, projectID, "only-secondary")
		secondaryStream := transitionWritesStream(projectID, "only-secondary", 2)
		_, err = db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
			ObjectStream: secondaryStream,
			Encryption:   metabasetest.DefaultEncryption,
		})
		require.NoError(t, err)
		commitInlineSegment(secondaryStream)

		secondarySegments, err := secondary.TestingGetAllSegments(ctx, metabase.NewNodeAliasCache(secondary, true))
		require.NoError(t, err)
		require.Len(t, secondarySegments, 1)
		require.Equal(t, secondaryStream.StreamID, secondarySegments[0].StreamID)
	})
}

// transitionWritesSeedLockObject seeds a committed object with the given retention and
// legal hold into the given backend, returning its stream.
func transitionWritesSeedLockObject(ctx *testcontext.Context, t *testing.T, adapter metabase.Adapter, projectID uuid.UUID, key string, retention metabase.Retention, legalHold bool) metabase.ObjectStream {
	stream := transitionWritesStream(projectID, key, 1)
	transitionWritesSeedRaw(ctx, t, adapter, metabase.RawObject{
		ObjectStream: stream,
		Retention:    retention,
		LegalHold:    legalHold,
	})
	return stream
}

// TestTransitionWrites_SetObjectExactVersionRetention verifies retention mutations of
// an exact version route to the owning backend and leave the other untouched.
func TestTransitionWrites_SetObjectExactVersionRetention(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		future := time.Now().Add(time.Hour)
		retention := metabase.Retention{Mode: storj.ComplianceMode, RetainUntil: future}
		extended := metabase.Retention{Mode: storj.ComplianceMode, RetainUntil: future.Add(time.Hour)}

		// Object in primary -> mutated in primary.
		primaryStream := transitionWritesSeedLockObject(ctx, t, primary, projectID, "in-primary", retention, false)
		require.NoError(t, db.SetObjectExactVersionRetention(ctx, metabase.SetObjectExactVersionRetention{
			ObjectLocation: primaryStream.Location(),
			Version:        primaryStream.Version,
			Retention:      extended,
		}))
		gotPrimary, err := primary.GetObjectExactVersionRetention(ctx, metabase.GetObjectExactVersionRetention{
			ObjectLocation: primaryStream.Location(),
			Version:        primaryStream.Version,
		})
		require.NoError(t, err)
		require.WithinDuration(t, extended.RetainUntil, gotPrimary.RetainUntil, time.Second)

		// Object in secondary -> fallback to secondary; primary untouched.
		secondaryStream := transitionWritesSeedLockObject(ctx, t, secondary, projectID, "in-secondary", retention, false)
		require.NoError(t, db.SetObjectExactVersionRetention(ctx, metabase.SetObjectExactVersionRetention{
			ObjectLocation: secondaryStream.Location(),
			Version:        secondaryStream.Version,
			Retention:      extended,
		}))
		gotSecondary, err := secondary.GetObjectExactVersionRetention(ctx, metabase.GetObjectExactVersionRetention{
			ObjectLocation: secondaryStream.Location(),
			Version:        secondaryStream.Version,
		})
		require.NoError(t, err)
		require.WithinDuration(t, extended.RetainUntil, gotSecondary.RetainUntil, time.Second)

		_, err = primary.GetObjectExactVersionRetention(ctx, metabase.GetObjectExactVersionRetention{
			ObjectLocation: secondaryStream.Location(),
			Version:        secondaryStream.Version,
		})
		require.True(t, metabase.ErrObjectNotFound.Has(err))
	})
}

// TestTransitionWrites_SetObjectLastCommittedRetention verifies last-committed
// retention mutations route to the owning backend.
func TestTransitionWrites_SetObjectLastCommittedRetention(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		future := time.Now().Add(time.Hour)
		retention := metabase.Retention{Mode: storj.ComplianceMode, RetainUntil: future}
		extended := metabase.Retention{Mode: storj.ComplianceMode, RetainUntil: future.Add(time.Hour)}

		// Object in primary -> mutated in primary.
		primaryStream := transitionWritesSeedLockObject(ctx, t, primary, projectID, "in-primary", retention, false)
		require.NoError(t, db.SetObjectLastCommittedRetention(ctx, metabase.SetObjectLastCommittedRetention{
			ObjectLocation: primaryStream.Location(),
			Retention:      extended,
		}))
		gotPrimary, err := primary.GetObjectLastCommittedRetention(ctx, metabase.GetObjectLastCommittedRetention{
			ObjectLocation: primaryStream.Location(),
		})
		require.NoError(t, err)
		require.WithinDuration(t, extended.RetainUntil, gotPrimary.RetainUntil, time.Second)

		// Object in secondary -> fallback; primary untouched.
		secondaryStream := transitionWritesSeedLockObject(ctx, t, secondary, projectID, "in-secondary", retention, false)
		require.NoError(t, db.SetObjectLastCommittedRetention(ctx, metabase.SetObjectLastCommittedRetention{
			ObjectLocation: secondaryStream.Location(),
			Retention:      extended,
		}))
		gotSecondary, err := secondary.GetObjectLastCommittedRetention(ctx, metabase.GetObjectLastCommittedRetention{
			ObjectLocation: secondaryStream.Location(),
		})
		require.NoError(t, err)
		require.WithinDuration(t, extended.RetainUntil, gotSecondary.RetainUntil, time.Second)

		_, err = primary.GetObjectLastCommittedRetention(ctx, metabase.GetObjectLastCommittedRetention{
			ObjectLocation: secondaryStream.Location(),
		})
		require.True(t, metabase.ErrObjectNotFound.Has(err))
	})
}

// TestTransitionWrites_SetObjectExactVersionLegalHold verifies legal-hold mutations of
// an exact version route to the owning backend.
func TestTransitionWrites_SetObjectExactVersionLegalHold(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		// Object in primary -> mutated in primary.
		primaryStream := transitionWritesSeedLockObject(ctx, t, primary, projectID, "in-primary", metabase.Retention{}, false)
		require.NoError(t, db.SetObjectExactVersionLegalHold(ctx, metabase.SetObjectExactVersionLegalHold{
			ObjectLocation: primaryStream.Location(),
			Version:        primaryStream.Version,
			Enabled:        true,
		}))
		heldPrimary, err := primary.GetObjectExactVersionLegalHold(ctx, metabase.GetObjectExactVersionLegalHold{
			ObjectLocation: primaryStream.Location(),
			Version:        primaryStream.Version,
		})
		require.NoError(t, err)
		require.True(t, heldPrimary)

		// Object in secondary -> fallback; primary untouched.
		secondaryStream := transitionWritesSeedLockObject(ctx, t, secondary, projectID, "in-secondary", metabase.Retention{}, false)
		require.NoError(t, db.SetObjectExactVersionLegalHold(ctx, metabase.SetObjectExactVersionLegalHold{
			ObjectLocation: secondaryStream.Location(),
			Version:        secondaryStream.Version,
			Enabled:        true,
		}))
		heldSecondary, err := secondary.GetObjectExactVersionLegalHold(ctx, metabase.GetObjectExactVersionLegalHold{
			ObjectLocation: secondaryStream.Location(),
			Version:        secondaryStream.Version,
		})
		require.NoError(t, err)
		require.True(t, heldSecondary)

		_, err = primary.GetObjectExactVersionLegalHold(ctx, metabase.GetObjectExactVersionLegalHold{
			ObjectLocation: secondaryStream.Location(),
			Version:        secondaryStream.Version,
		})
		require.True(t, metabase.ErrObjectNotFound.Has(err))
	})
}

// TestTransitionWrites_SetObjectLastCommittedLegalHold verifies last-committed
// legal-hold mutations route to the owning backend.
func TestTransitionWrites_SetObjectLastCommittedLegalHold(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		// Object in primary -> mutated in primary.
		primaryStream := transitionWritesSeedLockObject(ctx, t, primary, projectID, "in-primary", metabase.Retention{}, false)
		require.NoError(t, db.SetObjectLastCommittedLegalHold(ctx, metabase.SetObjectLastCommittedLegalHold{
			ObjectLocation: primaryStream.Location(),
			Enabled:        true,
		}))
		heldPrimary, err := primary.GetObjectLastCommittedLegalHold(ctx, metabase.GetObjectLastCommittedLegalHold{
			ObjectLocation: primaryStream.Location(),
		})
		require.NoError(t, err)
		require.True(t, heldPrimary)

		// Object in secondary -> fallback; primary untouched.
		secondaryStream := transitionWritesSeedLockObject(ctx, t, secondary, projectID, "in-secondary", metabase.Retention{}, false)
		require.NoError(t, db.SetObjectLastCommittedLegalHold(ctx, metabase.SetObjectLastCommittedLegalHold{
			ObjectLocation: secondaryStream.Location(),
			Enabled:        true,
		}))
		heldSecondary, err := secondary.GetObjectLastCommittedLegalHold(ctx, metabase.GetObjectLastCommittedLegalHold{
			ObjectLocation: secondaryStream.Location(),
		})
		require.NoError(t, err)
		require.True(t, heldSecondary)

		_, err = primary.GetObjectLastCommittedLegalHold(ctx, metabase.GetObjectLastCommittedLegalHold{
			ObjectLocation: secondaryStream.Location(),
		})
		require.True(t, metabase.ErrObjectNotFound.Has(err))
	})
}

// TestTransitionWrites_UpdateObjectLastCommittedMetadata verifies metadata updates
// route to the owning backend and change metadata only there.
func TestTransitionWrites_UpdateObjectLastCommittedMetadata(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		newMetadata := func() metabase.EncryptedUserData {
			return metabase.EncryptedUserData{
				EncryptedMetadata:             testrand.Bytes(32),
				EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
				EncryptedMetadataEncryptedKey: testrand.Bytes(32),
			}
		}
		readMetadata := func(adapter metabase.Adapter, stream metabase.ObjectStream) []byte {
			obj, err := adapter.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
				ObjectLocation: stream.Location(),
				Version:        stream.Version,
			})
			require.NoError(t, err)
			return obj.EncryptedMetadata
		}

		// Object in primary -> metadata changed in primary.
		primaryStream := transitionSeedCommitted(ctx, t, primary, projectID, "in-primary")
		primaryData := newMetadata()
		require.NoError(t, db.UpdateObjectLastCommittedMetadata(ctx, metabase.UpdateObjectLastCommittedMetadata{
			ObjectLocation:    primaryStream.Location(),
			StreamID:          primaryStream.StreamID,
			EncryptedUserData: primaryData,
			Includes:          metabase.EncryptedUserDataIncludes{Metadata: true},
		}))
		require.Equal(t, primaryData.EncryptedMetadata, readMetadata(primary, primaryStream))

		// Object in secondary -> fallback; metadata changed only in secondary.
		secondaryStream := transitionSeedCommitted(ctx, t, secondary, projectID, "in-secondary")
		secondaryData := newMetadata()
		require.NoError(t, db.UpdateObjectLastCommittedMetadata(ctx, metabase.UpdateObjectLastCommittedMetadata{
			ObjectLocation:    secondaryStream.Location(),
			StreamID:          secondaryStream.StreamID,
			EncryptedUserData: secondaryData,
			Includes:          metabase.EncryptedUserDataIncludes{Metadata: true},
		}))
		require.Equal(t, secondaryData.EncryptedMetadata, readMetadata(secondary, secondaryStream))

		_, err := primary.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
			ObjectLocation: secondaryStream.Location(),
			Version:        secondaryStream.Version,
		})
		require.True(t, metabase.ErrObjectNotFound.Has(err))
	})
}
