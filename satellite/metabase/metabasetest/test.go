// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"bytes"
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// BeginObjectNextVersion is for testing metabase.BeginObjectNextVersion.
type BeginObjectNextVersion struct {
	Opts     metabase.BeginObjectNextVersion
	Version  metabase.Version
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step BeginObjectNextVersion) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) metabase.Object {
	got, err := db.BeginObjectNextVersion(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	if step.ErrClass == nil {
		if step.Version != 0 {
			require.Equal(t, step.Version, got.Version)
		}
		require.WithinDuration(t, time.Now(), got.CreatedAt, 5*time.Second)

		require.Equal(t, step.Opts.ObjectStream.ProjectID, got.ObjectStream.ProjectID)
		require.Equal(t, step.Opts.ObjectStream.BucketName, got.ObjectStream.BucketName)
		require.Equal(t, step.Opts.ObjectStream.ObjectKey, got.ObjectStream.ObjectKey)
		require.Equal(t, step.Opts.ObjectStream.StreamID, got.ObjectStream.StreamID)
		require.Equal(t, metabase.Pending, got.Status)

		require.Equal(t, step.Opts.ExpiresAt, got.ExpiresAt)

		gotDeadline := got.ZombieDeletionDeadline
		optsDeadline := step.Opts.ZombieDeletionDeadline
		if optsDeadline == nil {
			require.WithinDuration(t, time.Now().Add(24*time.Hour), *gotDeadline, 5*time.Second)
		} else {
			require.WithinDuration(t, *optsDeadline, *gotDeadline, 5*time.Second)
		}
		require.Equal(t, step.Opts.Encryption, got.Encryption)
	}
	return got
}

// BeginObjectExactVersion is for testing metabase.BeginObjectExactVersion.
type BeginObjectExactVersion struct {
	Opts     metabase.BeginObjectExactVersion
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step BeginObjectExactVersion) Check(ctx *testcontext.Context, t require.TestingT, db *metabase.DB) metabase.Object {
	got, err := db.BeginObjectExactVersion(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	if step.ErrClass == nil {
		require.Equal(t, step.Opts.Version, got.Version)
		require.WithinDuration(t, time.Now(), got.CreatedAt, 5*time.Second)
		require.Equal(t, step.Opts.ObjectStream, got.ObjectStream)
		require.Equal(t, step.Opts.ExpiresAt, got.ExpiresAt)

		gotDeadline := got.ZombieDeletionDeadline
		optsDeadline := step.Opts.ZombieDeletionDeadline
		if optsDeadline == nil {
			require.WithinDuration(t, time.Now().Add(24*time.Hour), *gotDeadline, 5*time.Second)
		} else {
			require.WithinDuration(t, *optsDeadline, *gotDeadline, 5*time.Second)
		}
		require.Equal(t, step.Opts.Encryption, got.Encryption)
	}
	return got
}

// CommitObject is for testing metabase.CommitObject.
type CommitObject struct {
	Opts          metabase.CommitObject
	ExpectVersion metabase.Version
	ErrClass      *errs.Class
	ErrText       string
}

// Check runs the test.
func (step CommitObject) Check(ctx *testcontext.Context, t require.TestingT, db *metabase.DB) metabase.Object {
	object, err := db.CommitObject(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	if err == nil {
		if step.ExpectVersion == 0 {
			// ignore the version check when not specified
			step.Opts.ObjectStream.Version = object.Version
		} else {
			step.Opts.ObjectStream.Version = step.ExpectVersion
		}
		require.Equal(t, step.Opts.ObjectStream, object.ObjectStream)
	}
	return object
}

// CommitInlineObject is for testing metabase.CommitInlineObject.
type CommitInlineObject struct {
	Opts          metabase.CommitInlineObject
	ExpectVersion metabase.Version
	ErrClass      *errs.Class
	ErrText       string
}

// Check runs the test.
func (step CommitInlineObject) Check(ctx *testcontext.Context, t require.TestingT, db *metabase.DB) metabase.Object {
	object, err := db.CommitInlineObject(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	if err == nil {
		if step.ExpectVersion == 0 {
			// Ignore version check when not specified.
			step.Opts.ObjectStream.Version = object.Version
		} else {
			step.Opts.ObjectStream.Version = step.ExpectVersion
		}
		require.Equal(t, step.Opts.ObjectStream, object.ObjectStream)
	}
	return object
}

// BeginSegment is for testing metabase.BeginSegment.
type BeginSegment struct {
	Opts     metabase.BeginSegment
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step BeginSegment) Check(ctx *testcontext.Context, t require.TestingT, db *metabase.DB) {
	err := db.BeginSegment(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// CommitSegment is for testing metabase.CommitSegment.
type CommitSegment struct {
	Opts     metabase.CommitSegment
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step CommitSegment) Check(ctx *testcontext.Context, t require.TestingT, db *metabase.DB) {
	err := db.CommitSegment(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// CommitInlineSegment is for testing metabase.CommitInlineSegment.
type CommitInlineSegment struct {
	Opts     metabase.CommitInlineSegment
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step CommitInlineSegment) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.CommitInlineSegment(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// DeleteAllBucketObjects is for testing metabase.DeleteAllBucketObjects.
type DeleteAllBucketObjects struct {
	Opts     metabase.DeleteAllBucketObjects
	Deleted  int64
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step DeleteAllBucketObjects) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	deleted, err := db.DeleteAllBucketObjects(ctx, step.Opts)
	require.Equal(t, step.Deleted, deleted)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// UncoordinatedDeleteAllBucketObjects is for testing metabase.UncoordinatedDeleteAllBucketObjects.
type UncoordinatedDeleteAllBucketObjects struct {
	Opts     metabase.UncoordinatedDeleteAllBucketObjects
	Deleted  int64
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step UncoordinatedDeleteAllBucketObjects) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	deleted, err := db.UncoordinatedDeleteAllBucketObjects(ctx, step.Opts)
	require.Equal(t, step.Deleted, deleted)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// UpdateObjectLastCommittedMetadata is for testing metabase.UpdateObjectLastCommittedMetadata.
type UpdateObjectLastCommittedMetadata struct {
	Opts     metabase.UpdateObjectLastCommittedMetadata
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step UpdateObjectLastCommittedMetadata) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.UpdateObjectLastCommittedMetadata(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// UpdateSegmentPieces is for testing metabase.UpdateSegmentPieces.
type UpdateSegmentPieces struct {
	Opts     metabase.UpdateSegmentPieces
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step UpdateSegmentPieces) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.UpdateSegmentPieces(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// GetObjectExactVersion is for testing metabase.GetObjectExactVersion.
type GetObjectExactVersion struct {
	Opts     metabase.GetObjectExactVersion
	Result   metabase.Object
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step GetObjectExactVersion) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetObjectExactVersion(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff())
	require.Zero(t, diff)
}

// GetObjectLastCommitted is for testing metabase.GetObjectLastCommitted.
type GetObjectLastCommitted struct {
	Opts     metabase.GetObjectLastCommitted
	Result   metabase.Object
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step GetObjectLastCommitted) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetObjectLastCommitted(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

// GetSegmentByPosition is for testing metabase.GetSegmentByPosition.
type GetSegmentByPosition struct {
	Opts     metabase.GetSegmentByPosition
	Result   metabase.Segment
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step GetSegmentByPosition) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetSegmentByPosition(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff())
	require.Zero(t, diff)
}

// GetLatestObjectLastSegment is for testing metabase.GetLatestObjectLastSegment.
type GetLatestObjectLastSegment struct {
	Opts     metabase.GetLatestObjectLastSegment
	Result   metabase.Segment
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step GetLatestObjectLastSegment) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetLatestObjectLastSegment(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff())
	require.Zero(t, diff)
}

// BucketEmpty is for testing metabase.BucketEmpty.
type BucketEmpty struct {
	Opts     metabase.BucketEmpty
	Result   bool
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step BucketEmpty) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.BucketEmpty(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	require.Equal(t, step.Result, result)
}

// ListSegments is for testing metabase.ListSegments.
type ListSegments struct {
	Opts     metabase.ListSegments
	Result   metabase.ListSegmentsResult
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step ListSegments) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.ListSegments(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	if len(step.Result.Segments) == 0 && len(result.Segments) == 0 {
		return
	}

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff())
	require.Zero(t, diff)
}

// ListVerifySegments is for testing metabase.ListVerifySegments.
type ListVerifySegments struct {
	Opts     metabase.ListVerifySegments
	Result   metabase.ListVerifySegmentsResult
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step ListVerifySegments) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.ListVerifySegments(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff(), cmpopts.EquateEmpty())
	require.Zero(t, diff)
}

// ListObjects is for testing metabase.ListObjects.
type ListObjects struct {
	Opts     metabase.ListObjects
	Result   metabase.ListObjectsResult
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step ListObjects) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.ListObjects(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff(), cmpopts.EquateEmpty())
	require.Zero(t, diff)
}

// ListStreamPositions is for testing metabase.ListStreamPositions.
type ListStreamPositions struct {
	Opts     metabase.ListStreamPositions
	Result   metabase.ListStreamPositionsResult
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step ListStreamPositions) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.ListStreamPositions(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff())
	require.Zero(t, diff)
}

// GetStreamPieceCountByNodeID is for testing metabase.GetStreamPieceCountByNodeID.
type GetStreamPieceCountByNodeID struct {
	Opts     metabase.GetStreamPieceCountByNodeID
	Result   map[storj.NodeID]int64
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step GetStreamPieceCountByNodeID) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetStreamPieceCountByNodeID(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result)
	require.Zero(t, diff)
}

// IterateLoopSegments is for testing metabase.IterateLoopSegments.
type IterateLoopSegments struct {
	Opts     metabase.IterateLoopSegments
	Result   []metabase.LoopSegmentEntry
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step IterateLoopSegments) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result := make([]metabase.LoopSegmentEntry, 0, 10)
	err := db.IterateLoopSegments(ctx, step.Opts,
		func(ctx context.Context, iterator metabase.LoopSegmentsIterator) error {
			var entry metabase.LoopSegmentEntry
			for iterator.Next(ctx, &entry) {
				result = append(result, entry)
			}
			return nil
		})
	checkError(t, err, step.ErrClass, step.ErrText)

	if len(result) == 0 {
		result = nil
	}

	sort.Slice(step.Result, func(i, j int) bool {
		if step.Result[i].StreamID == step.Result[j].StreamID {
			return step.Result[i].Position.Less(step.Result[j].Position)
		}
		return bytes.Compare(step.Result[i].StreamID[:], step.Result[j].StreamID[:]) < 0
	})
	// ignore AliasPieces because we won't be always able to predict node aliases for tests
	diff := cmp.Diff(step.Result, result, DefaultTimeDiff(), cmpopts.IgnoreFields(metabase.LoopSegmentEntry{}, "AliasPieces"))
	require.Zero(t, diff)
}

// DeleteObjectExactVersion is for testing metabase.DeleteObjectExactVersion.
type DeleteObjectExactVersion struct {
	Opts     metabase.DeleteObjectExactVersion
	Result   metabase.DeleteObjectResult
	ErrClass *errs.Class
	ErrText  string
}

func compareDeleteObjectResult(t testing.TB, got, exp metabase.DeleteObjectResult) {
	t.Helper()

	sortObjects(got.Markers)
	sortObjects(exp.Markers)
	if len(got.Markers) == len(exp.Markers) {
		// marker stream ID-s are internally generated, so we cannot upfront figure out what
		// the values are.
		for i := range got.Markers {
			exp.Markers[i].StreamID = got.Markers[i].StreamID
		}

		// ignore version checking if it's not provided.
		for i := range got.Markers {
			if exp.Markers[i].Version == 0 {
				exp.Markers[i].Version = got.Markers[i].Version
			}
		}
	}

	sortObjects(got.Removed)
	sortObjects(exp.Removed)

	diff := cmp.Diff(exp, got, DefaultTimeDiff(), cmpopts.EquateEmpty())
	require.Zero(t, diff)
}

// Check runs the test.
func (step DeleteObjectExactVersion) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.DeleteObjectExactVersion(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	compareDeleteObjectResult(t, result, step.Result)
}

// DeletePendingObject is for testing metabase.DeletePendingObject.
type DeletePendingObject struct {
	Opts     metabase.DeletePendingObject
	Result   metabase.DeleteObjectResult
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step DeletePendingObject) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.DeletePendingObject(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	compareDeleteObjectResult(t, result, step.Result)
}

// DeleteExpiredObjects is for testing metabase.DeleteExpiredObjects.
type DeleteExpiredObjects struct {
	Opts metabase.DeleteExpiredObjects

	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step DeleteExpiredObjects) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.DeleteExpiredObjects(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// DeleteZombieObjects is for testing metabase.DeleteZombieObjects.
type DeleteZombieObjects struct {
	Opts metabase.DeleteZombieObjects

	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step DeleteZombieObjects) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.DeleteZombieObjects(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// IterateCollector is for testing metabase.IterateCollector.
type IterateCollector []metabase.ObjectEntry

// Add adds object entries from iterator to the collection.
func (coll *IterateCollector) Add(ctx context.Context, it metabase.ObjectsIterator) error {
	var item metabase.ObjectEntry
	for it.Next(ctx, &item) {
		*coll = append(*coll, item)
	}
	return nil
}

// PendingObjectsCollector is for testing metabase.PendingObjectsCollector.
type PendingObjectsCollector []metabase.PendingObjectEntry

// Add adds object entries from iterator to the collection.
func (coll *PendingObjectsCollector) Add(ctx context.Context, it metabase.PendingObjectsIterator) error {
	var item metabase.PendingObjectEntry
	for it.Next(ctx, &item) {
		*coll = append(*coll, item)
	}
	return nil
}

// IteratePendingObjectsByKey is for testing metabase.IteratePendingObjectsByKey.
type IteratePendingObjectsByKey struct {
	Opts metabase.IteratePendingObjectsByKey

	Result   []metabase.ObjectEntry
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step IteratePendingObjectsByKey) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	var collector IterateCollector

	err := db.IteratePendingObjectsByKey(ctx, step.Opts, collector.Add)
	checkError(t, err, step.ErrClass, step.ErrText)

	result := []metabase.ObjectEntry(collector)

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff())
	require.Zero(t, diff)
}

// IterateObjectsWithStatus is for testing metabase.IterateObjectsWithStatus.
type IterateObjectsWithStatus struct {
	Opts metabase.IterateObjectsWithStatus

	Result   []metabase.ObjectEntry
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step IterateObjectsWithStatus) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	var result IterateCollector

	err := db.IterateObjectsAllVersionsWithStatus(ctx, step.Opts, result.Add)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, []metabase.ObjectEntry(result), DefaultTimeDiff(),
		// Iterators don't implement IsLatest.
		cmpopts.IgnoreFields(metabase.ObjectEntry{}, "IsLatest"),
	)
	require.Zero(t, diff)
}

// IterateObjectsWithStatusAscending is for testing metabase.IterateObjectsWithStatusAscending.
type IterateObjectsWithStatusAscending struct {
	Opts metabase.IterateObjectsWithStatus

	Result   []metabase.ObjectEntry
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step IterateObjectsWithStatusAscending) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	var result IterateCollector

	err := db.IterateObjectsAllVersionsWithStatusAscending(ctx, step.Opts, result.Add)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, []metabase.ObjectEntry(result), DefaultTimeDiff(),
		// Iterators don't implement IsLatest.
		cmpopts.IgnoreFields(metabase.ObjectEntry{}, "IsLatest"),
	)
	require.Zero(t, diff)
}

// EnsureNodeAliases is for testing metabase.EnsureNodeAliases.
type EnsureNodeAliases struct {
	Opts metabase.EnsureNodeAliases

	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step EnsureNodeAliases) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.EnsureNodeAliases(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// ListNodeAliases is for testing metabase.ListNodeAliases.
type ListNodeAliases struct {
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step ListNodeAliases) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) []metabase.NodeAliasEntry {
	result, err := db.ListNodeAliases(ctx)
	checkError(t, err, step.ErrClass, step.ErrText)
	return result
}

// GetNodeAliasEntries is for testing metabase.GetNodeAliasEntries.
type GetNodeAliasEntries struct {
	Opts     metabase.GetNodeAliasEntries
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step GetNodeAliasEntries) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) []metabase.NodeAliasEntry {
	result, err := db.GetNodeAliasEntries(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	return result
}

// GetTableStats is for testing metabase.GetTableStats.
type GetTableStats struct {
	Opts     metabase.GetTableStats
	Result   metabase.TableStats
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step GetTableStats) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) metabase.TableStats {
	result, err := db.GetTableStats(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result)
	require.Zero(t, diff)

	return result
}

// BeginMoveObject is for testing metabase.BeginMoveObject.
type BeginMoveObject struct {
	Opts     metabase.BeginMoveObject
	Result   metabase.BeginMoveObjectResult
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step BeginMoveObject) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.BeginMoveObject(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result)
	require.Zero(t, diff)
}

// FinishMoveObject is for testing metabase.FinishMoveObject.
type FinishMoveObject struct {
	Opts     metabase.FinishMoveObject
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step FinishMoveObject) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.FinishMoveObject(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// BeginCopyObject is for testing metabase.BeginCopyObject.
type BeginCopyObject struct {
	Opts     metabase.BeginCopyObject
	Result   metabase.BeginCopyObjectResult
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step BeginCopyObject) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.BeginCopyObject(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result)
	require.Zero(t, diff)
}

// FinishCopyObject is for testing metabase.FinishCopyObject.
type FinishCopyObject struct {
	Opts     metabase.FinishCopyObject
	Result   metabase.Object
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step FinishCopyObject) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) metabase.Object {
	result, err := db.FinishCopyObject(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	// ignore version checking if it's not provided.
	if step.Result.Version == 0 {
		step.Result.Version = result.Version
	}

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff())
	require.Zero(t, diff)
	return result
}

// DeleteObjectLastCommitted is for testing metabase.DeleteObjectLastCommitted.
type DeleteObjectLastCommitted struct {
	Opts   metabase.DeleteObjectLastCommitted
	Result metabase.DeleteObjectResult

	ErrClass *errs.Class
	ErrText  string

	OutputMarkerStreamID *uuid.UUID
}

// Check runs the test.
func (step DeleteObjectLastCommitted) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) metabase.DeleteObjectResult {
	result, err := db.DeleteObjectLastCommitted(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	compareDeleteObjectResult(t, result, step.Result)

	if step.OutputMarkerStreamID != nil && len(result.Markers) > 0 {
		*step.OutputMarkerStreamID = result.Markers[0].StreamID
	}

	return result
}

// DeleteObjects contains options for testing the (*metabase.DB).DeleteObjects method.
type DeleteObjects struct {
	Opts   metabase.DeleteObjects
	Result metabase.DeleteObjectsResult

	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step DeleteObjects) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.DeleteObjects(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	// Marker stream IDs are internally generated, so we cannot upfront figure out what their values are.
	for _, item := range result.Items {
		if item.Marker != nil {
			item.Marker.StreamVersionID.SetStreamID(uuid.UUID{})
		}
	}

	diff := cmp.Diff(step.Result, result)
	require.Zero(t, diff)
}

// CollectBucketTallies is for testing metabase.CollectBucketTallies.
type CollectBucketTallies struct {
	Opts     metabase.CollectBucketTallies
	Result   []metabase.BucketTally
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step CollectBucketTallies) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.CollectBucketTallies(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	sortBucketTallies(result)
	sortBucketTallies(step.Result)

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff(), cmpopts.EquateEmpty())
	require.Zero(t, diff)
}

// GetObjectExactVersionLegalHold is for testing metabase.GetObjectExactVersionLegalHold.
type GetObjectExactVersionLegalHold struct {
	Opts     metabase.GetObjectExactVersionLegalHold
	Result   bool
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step GetObjectExactVersionLegalHold) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetObjectExactVersionLegalHold(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff())
	require.Zero(t, diff)
}

// GetObjectLastCommittedLegalHold is for testing metabase.GetObjectLastCommittedLegalHold.
type GetObjectLastCommittedLegalHold struct {
	Opts     metabase.GetObjectLastCommittedLegalHold
	Result   bool
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step GetObjectLastCommittedLegalHold) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetObjectLastCommittedLegalHold(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff())
	require.Zero(t, diff)
}

// GetObjectExactVersionRetention is for testing metabase.GetObjectExactVersionRetention.
type GetObjectExactVersionRetention struct {
	Opts     metabase.GetObjectExactVersionRetention
	Result   metabase.Retention
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step GetObjectExactVersionRetention) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetObjectExactVersionRetention(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff())
	require.Zero(t, diff)
}

// GetObjectLastCommittedRetention is for testing metabase.GetObjectLastCommittedRetention.
type GetObjectLastCommittedRetention struct {
	Opts     metabase.GetObjectLastCommittedRetention
	Result   metabase.Retention
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step GetObjectLastCommittedRetention) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetObjectLastCommittedRetention(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, DefaultTimeDiff())
	require.Zero(t, diff)
}

// SetObjectExactVersionRetention is for testing metabase.SetObjectExactVersionRetention.
type SetObjectExactVersionRetention struct {
	Opts     metabase.SetObjectExactVersionRetention
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step SetObjectExactVersionRetention) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.SetObjectExactVersionRetention(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// SetObjectLastCommittedRetention is for testing metabase.SetObjectLastCommittedRetention.
type SetObjectLastCommittedRetention struct {
	Opts     metabase.SetObjectLastCommittedRetention
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step SetObjectLastCommittedRetention) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.SetObjectLastCommittedRetention(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// SetObjectExactVersionLegalHold is for testing metabase.SetObjectExactVersionLegalHold.
type SetObjectExactVersionLegalHold struct {
	Opts     metabase.SetObjectExactVersionLegalHold
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step SetObjectExactVersionLegalHold) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.SetObjectExactVersionLegalHold(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

// SetObjectLastCommittedLegalHold is for testing metabase.SetObjectLastCommittedLegalHold.
type SetObjectLastCommittedLegalHold struct {
	Opts     metabase.SetObjectLastCommittedLegalHold
	ErrClass *errs.Class
	ErrText  string
}

// Check runs the test.
func (step SetObjectLastCommittedLegalHold) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.SetObjectLastCommittedLegalHold(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}
