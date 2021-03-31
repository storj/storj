// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

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
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
)

type BeginObjectNextVersion struct {
	Opts     metabase.BeginObjectNextVersion
	Version  metabase.Version
	ErrClass *errs.Class
	ErrText  string
}

func (step BeginObjectNextVersion) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	got, err := db.BeginObjectNextVersion(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	require.Equal(t, step.Version, got)
}

type BeginObjectExactVersion struct {
	Opts     metabase.BeginObjectExactVersion
	Version  metabase.Version
	ErrClass *errs.Class
	ErrText  string
}

func (step BeginObjectExactVersion) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	got, err := db.BeginObjectExactVersion(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	if step.ErrClass == nil {
		require.Equal(t, step.Version, got.Version)
		require.WithinDuration(t, time.Now(), got.CreatedAt, 5*time.Second)
		require.Equal(t, step.Opts.ObjectStream, got.ObjectStream)
		require.Equal(t, step.Opts.ExpiresAt, got.ExpiresAt)
		require.Equal(t, step.Opts.ZombieDeletionDeadline, got.ZombieDeletionDeadline)
		require.Equal(t, step.Opts.Encryption, got.Encryption)
	}
}

type CommitObject struct {
	Opts     metabase.CommitObject
	ErrClass *errs.Class
	ErrText  string
}

func (step CommitObject) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) metabase.Object {
	object, err := db.CommitObject(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	if err == nil {
		require.Equal(t, step.Opts.ObjectStream, object.ObjectStream)
	}
	return object
}

type CommitObjectWithSegments struct {
	Opts     metabase.CommitObjectWithSegments
	Deleted  []metabase.DeletedSegmentInfo
	ErrClass *errs.Class
	ErrText  string
}

func (step CommitObjectWithSegments) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) metabase.Object {
	object, deleted, err := db.CommitObjectWithSegments(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	if err == nil {
		require.Equal(t, step.Opts.ObjectStream, object.ObjectStream)
	}
	require.Equal(t, step.Deleted, deleted)
	return object
}

type BeginSegment struct {
	Opts     metabase.BeginSegment
	ErrClass *errs.Class
	ErrText  string
}

func (step BeginSegment) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.BeginSegment(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

type CommitSegment struct {
	Opts     metabase.CommitSegment
	ErrClass *errs.Class
	ErrText  string
}

func (step CommitSegment) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.CommitSegment(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

type CommitInlineSegment struct {
	Opts     metabase.CommitInlineSegment
	ErrClass *errs.Class
	ErrText  string
}

func (step CommitInlineSegment) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.CommitInlineSegment(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

type DeleteBucketObjects struct {
	Opts     metabase.DeleteBucketObjects
	Deleted  int64
	ErrClass *errs.Class
	ErrText  string
}

func (step DeleteBucketObjects) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	deleted, err := db.DeleteBucketObjects(ctx, step.Opts)
	require.Equal(t, step.Deleted, deleted)
	checkError(t, err, step.ErrClass, step.ErrText)
}

type UpdateObjectMetadata struct {
	Opts     metabase.UpdateObjectMetadata
	ErrClass *errs.Class
	ErrText  string
}

func (step UpdateObjectMetadata) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.UpdateObjectMetadata(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

type UpdateSegmentPieces struct {
	Opts     metabase.UpdateSegmentPieces
	ErrClass *errs.Class
	ErrText  string
}

func (step UpdateSegmentPieces) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.UpdateSegmentPieces(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

type GetObjectExactVersion struct {
	Opts     metabase.GetObjectExactVersion
	Result   metabase.Object
	ErrClass *errs.Class
	ErrText  string
}

func (step GetObjectExactVersion) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetObjectExactVersion(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type GetObjectLatestVersion struct {
	Opts     metabase.GetObjectLatestVersion
	Result   metabase.Object
	ErrClass *errs.Class
	ErrText  string
}

func (step GetObjectLatestVersion) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetObjectLatestVersion(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type GetSegmentByLocation struct {
	Opts     metabase.GetSegmentByLocation
	Result   metabase.Segment
	ErrClass *errs.Class
	ErrText  string
}

func (step GetSegmentByLocation) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetSegmentByLocation(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type GetSegmentByPosition struct {
	Opts     metabase.GetSegmentByPosition
	Result   metabase.Segment
	ErrClass *errs.Class
	ErrText  string
}

func (step GetSegmentByPosition) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetSegmentByPosition(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type GetLatestObjectLastSegment struct {
	Opts     metabase.GetLatestObjectLastSegment
	Result   metabase.Segment
	ErrClass *errs.Class
	ErrText  string
}

func (step GetLatestObjectLastSegment) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetLatestObjectLastSegment(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type GetSegmentByOffset struct {
	Opts     metabase.GetSegmentByOffset
	Result   metabase.Segment
	ErrClass *errs.Class
	ErrText  string
}

func (step GetSegmentByOffset) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetSegmentByOffset(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type BucketEmpty struct {
	Opts     metabase.BucketEmpty
	Result   bool
	ErrClass *errs.Class
	ErrText  string
}

func (step BucketEmpty) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.BucketEmpty(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	require.Equal(t, step.Result, result)
}

type ListSegments struct {
	Opts     metabase.ListSegments
	Result   metabase.ListSegmentsResult
	ErrClass *errs.Class
	ErrText  string
}

func (step ListSegments) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.ListSegments(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type ListStreamPositions struct {
	Opts     metabase.ListStreamPositions
	Result   metabase.ListStreamPositionsResult
	ErrClass *errs.Class
	ErrText  string
}

func (step ListStreamPositions) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.ListStreamPositions(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type GetStreamPieceCountByNodeID struct {
	Opts     metabase.GetStreamPieceCountByNodeID
	Result   map[storj.NodeID]int64
	ErrClass *errs.Class
	ErrText  string
}

func (step GetStreamPieceCountByNodeID) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.GetStreamPieceCountByNodeID(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result)
	require.Zero(t, diff)
}

type IterateLoopStreams struct {
	Opts     metabase.IterateLoopStreams
	Result   map[uuid.UUID][]metabase.LoopSegmentEntry
	ErrClass *errs.Class
	ErrText  string
}

func (step IterateLoopStreams) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result := make(map[uuid.UUID][]metabase.LoopSegmentEntry)
	err := db.IterateLoopStreams(ctx, step.Opts,
		func(ctx context.Context, streamID uuid.UUID, next metabase.SegmentIterator) error {
			var segments []metabase.LoopSegmentEntry
			for {
				var segment metabase.LoopSegmentEntry
				if !next(&segment) {
					break
				}
				segments = append(segments, segment)
			}
			result[streamID] = segments
			return nil
		})
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type DeleteObjectExactVersion struct {
	Opts     metabase.DeleteObjectExactVersion
	Result   metabase.DeleteObjectResult
	ErrClass *errs.Class
	ErrText  string
}

func (step DeleteObjectExactVersion) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.DeleteObjectExactVersion(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type DeletePendingObject struct {
	Opts     metabase.DeletePendingObject
	Result   metabase.DeleteObjectResult
	ErrClass *errs.Class
	ErrText  string
}

func (step DeletePendingObject) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.DeletePendingObject(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type DeleteObjectLatestVersion struct {
	Opts     metabase.DeleteObjectLatestVersion
	Result   metabase.DeleteObjectResult
	ErrClass *errs.Class
	ErrText  string
}

func (step DeleteObjectLatestVersion) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.DeleteObjectLatestVersion(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type DeleteObjectAnyStatusAllVersions struct {
	Opts     metabase.DeleteObjectAnyStatusAllVersions
	Result   metabase.DeleteObjectResult
	ErrClass *errs.Class
	ErrText  string
}

func (step DeleteObjectAnyStatusAllVersions) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.DeleteObjectAnyStatusAllVersions(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type DeleteObjectsAllVersions struct {
	Opts     metabase.DeleteObjectsAllVersions
	Result   metabase.DeleteObjectResult
	ErrClass *errs.Class
	ErrText  string
}

func (step DeleteObjectsAllVersions) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	result, err := db.DeleteObjectsAllVersions(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	sortObjects(result.Objects)
	sortObjects(step.Result.Objects)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type DeleteExpiredObjects struct {
	ErrClass *errs.Class
	ErrText  string
}

func (step DeleteExpiredObjects) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.DeleteExpiredObjects(ctx, metabase.DeleteExpiredObjects{
		ExpiredBefore: time.Now(),
	})
	checkError(t, err, step.ErrClass, step.ErrText)
}

type IterateCollector []metabase.ObjectEntry

func (coll *IterateCollector) Add(ctx context.Context, it metabase.ObjectsIterator) error {
	var item metabase.ObjectEntry

	for it.Next(ctx, &item) {
		*coll = append(*coll, item)
	}
	return nil
}

type LoopIterateCollector []metabase.LoopObjectEntry

func (coll *LoopIterateCollector) Add(ctx context.Context, it metabase.LoopObjectsIterator) error {
	var item metabase.LoopObjectEntry

	for it.Next(ctx, &item) {
		*coll = append(*coll, item)
	}
	return nil
}

type IterateObjects struct {
	Opts metabase.IterateObjects

	Result   []metabase.ObjectEntry
	ErrClass *errs.Class
	ErrText  string
}

func (step IterateObjects) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	var collector IterateCollector

	err := db.IterateObjectsAllVersions(ctx, step.Opts, collector.Add)
	checkError(t, err, step.ErrClass, step.ErrText)

	result := []metabase.ObjectEntry(collector)
	sort.Slice(result, func(i, j int) bool {
		return result[i].ObjectKey < result[j].ObjectKey
	})
	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type IteratePendingObjectsByKey struct {
	Opts metabase.IteratePendingObjectsByKey

	Result   []metabase.ObjectEntry
	ErrClass *errs.Class
	ErrText  string
}

func (step IteratePendingObjectsByKey) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	var collector IterateCollector

	err := db.IteratePendingObjectsByKey(ctx, step.Opts, collector.Add)
	checkError(t, err, step.ErrClass, step.ErrText)

	result := []metabase.ObjectEntry(collector)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type IterateObjectsWithStatus struct {
	Opts metabase.IterateObjectsWithStatus

	Result   []metabase.ObjectEntry
	ErrClass *errs.Class
	ErrText  string
}

func (step IterateObjectsWithStatus) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	var result IterateCollector

	err := db.IterateObjectsAllVersionsWithStatus(ctx, step.Opts, result.Add)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, []metabase.ObjectEntry(result), cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type IterateLoopObjects struct {
	Opts metabase.IterateLoopObjects

	Result   []metabase.LoopObjectEntry
	ErrClass *errs.Class
	ErrText  string
}

func (step IterateLoopObjects) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	var result LoopIterateCollector

	err := db.IterateLoopObjects(ctx, step.Opts, result.Add)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, []metabase.LoopObjectEntry(result), cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type EnsureNodeAliases struct {
	Opts metabase.EnsureNodeAliases

	ErrClass *errs.Class
	ErrText  string
}

func (step EnsureNodeAliases) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.EnsureNodeAliases(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

type ListNodeAliases struct {
	ErrClass *errs.Class
	ErrText  string
}

func (step ListNodeAliases) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) []metabase.NodeAliasEntry {
	result, err := db.ListNodeAliases(ctx)
	checkError(t, err, step.ErrClass, step.ErrText)
	return result
}

func checkError(t testing.TB, err error, errClass *errs.Class, errText string) {
	if errClass != nil {
		require.True(t, errClass.Has(err), "expected an error %v got %v", *errClass, err)
	}
	if errText != "" {
		require.EqualError(t, err, errClass.New(errText).Error())
	}
	if errClass == nil && errText == "" {
		require.NoError(t, err)
	}
}

// less returns true if uuid 'id' is less than uuid 'other'.
func less(id uuid.UUID, other uuid.UUID) bool {
	for k, v := range id {
		if v < other[k] {
			return true
		} else if v > other[k] {
			return false
		}
	}
	return false
}

func sortObjects(objects []metabase.Object) {
	sort.Slice(objects, func(i, j int) bool {
		return bytes.Compare(objects[i].StreamID[:], objects[j].StreamID[:]) < 0
	})
}

func sortRawObjects(objects []metabase.RawObject) {
	sort.Slice(objects, func(i, j int) bool {
		return bytes.Compare(objects[i].StreamID[:], objects[j].StreamID[:]) < 0
	})
}

func sortRawSegments(segments []metabase.RawSegment) {
	sort.Slice(segments, func(i, j int) bool {
		return bytes.Compare(segments[i].StreamID[:], segments[j].StreamID[:]) < 0
	})
}

type DeleteAll struct{}

func (step DeleteAll) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	err := db.TestingDeleteAll(ctx)
	require.NoError(t, err)
}

type Verify metabase.RawState

func (step Verify) Check(ctx *testcontext.Context, t testing.TB, db *metabase.DB) {
	state, err := db.TestingGetState(ctx)
	require.NoError(t, err)

	sortRawObjects(state.Objects)
	sortRawObjects(step.Objects)
	sortRawSegments(state.Segments)
	sortRawSegments(step.Segments)

	diff := cmp.Diff(metabase.RawState(step), *state,
		cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type CreateTestObject struct {
	BeginObjectExactVersion *metabase.BeginObjectExactVersion
	CommitObject            *metabase.CommitObject
	// TODO add BeginSegment, CommitSegment
}

func (co CreateTestObject) Run(ctx *testcontext.Context, t testing.TB, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte) metabase.Object {
	boeOpts := metabase.BeginObjectExactVersion{
		ObjectStream: obj,
		Encryption:   defaultTestEncryption,
	}
	if co.BeginObjectExactVersion != nil {
		boeOpts = *co.BeginObjectExactVersion
	}

	BeginObjectExactVersion{
		Opts:    boeOpts,
		Version: obj.Version,
	}.Check(ctx, t, db)

	for i := byte(0); i < numberOfSegments; i++ {
		BeginSegment{
			Opts: metabase.BeginSegment{
				ObjectStream: obj,
				Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
				RootPieceID:  storj.PieceID{i + 1},
				Pieces: []metabase.Piece{{
					Number:      1,
					StorageNode: testrand.NodeID(),
				}},
			},
		}.Check(ctx, t, db)

		CommitSegment{
			Opts: metabase.CommitSegment{
				ObjectStream: obj,
				Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
				RootPieceID:  storj.PieceID{1},
				Pieces:       metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},

				EncryptedSize: 1060,
				PlainSize:     512,
				PlainOffset:   int64(i) * 512,
				Redundancy:    defaultTestRedundancy,
			},
		}.Check(ctx, t, db)
	}

	coOpts := metabase.CommitObject{
		ObjectStream: obj,
	}
	if co.CommitObject != nil {
		coOpts = *co.CommitObject
	}

	return CommitObject{
		Opts: coOpts,
	}.Check(ctx, t, db)
}

type invalidObjectLocation struct {
	Name           string
	ObjectLocation metabase.ObjectLocation
	ErrClass       *errs.Class
	ErrText        string
}

func invalidObjectLocations(base metabase.ObjectLocation) []invalidObjectLocation {
	var tests []invalidObjectLocation
	{
		location := base
		location.ProjectID = uuid.UUID{}
		tests = append(tests, invalidObjectLocation{
			Name:           "ProjectID missing",
			ObjectLocation: location,
			ErrClass:       &metabase.ErrInvalidRequest,
			ErrText:        "ProjectID missing",
		})
	}
	{
		location := base
		location.BucketName = ""
		tests = append(tests, invalidObjectLocation{
			Name:           "BucketName missing",
			ObjectLocation: location,
			ErrClass:       &metabase.ErrInvalidRequest,
			ErrText:        "BucketName missing",
		})
	}
	{
		location := base
		location.ObjectKey = ""
		tests = append(tests, invalidObjectLocation{
			Name:           "ObjectKey missing",
			ObjectLocation: location,
			ErrClass:       &metabase.ErrInvalidRequest,
			ErrText:        "ObjectKey missing",
		})
	}

	return tests
}

type invalidSegmentLocation struct {
	Name            string
	SegmentLocation metabase.SegmentLocation
	ErrClass        *errs.Class
	ErrText         string
}

func invalidSegmentLocations(base metabase.SegmentLocation) []invalidSegmentLocation {
	var tests []invalidSegmentLocation
	{
		location := base
		location.ProjectID = uuid.UUID{}
		tests = append(tests, invalidSegmentLocation{
			Name:            "ProjectID missing",
			SegmentLocation: location,
			ErrClass:        &metabase.ErrInvalidRequest,
			ErrText:         "ProjectID missing",
		})
	}
	{
		location := base
		location.BucketName = ""
		tests = append(tests, invalidSegmentLocation{
			Name:            "BucketName missing",
			SegmentLocation: location,
			ErrClass:        &metabase.ErrInvalidRequest,
			ErrText:         "BucketName missing",
		})
	}
	{
		location := base
		location.ObjectKey = ""
		tests = append(tests, invalidSegmentLocation{
			Name:            "ObjectKey missing",
			SegmentLocation: location,
			ErrClass:        &metabase.ErrInvalidRequest,
			ErrText:         "ObjectKey missing",
		})
	}

	return tests
}
