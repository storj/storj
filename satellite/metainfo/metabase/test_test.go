// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
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
	"storj.io/storj/satellite/metainfo/metabase"
)

type BeginObjectNextVersion struct {
	Opts     metabase.BeginObjectNextVersion
	Version  metabase.Version
	ErrClass *errs.Class
	ErrText  string
}

func (step BeginObjectNextVersion) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
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

func (step BeginObjectExactVersion) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	got, err := db.BeginObjectExactVersion(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	require.Equal(t, step.Version, got)
}

type CommitObject struct {
	Opts     metabase.CommitObject
	ErrClass *errs.Class
	ErrText  string
}

func (step CommitObject) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) metabase.Object {
	object, err := db.CommitObject(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
	if err == nil {
		require.Equal(t, step.Opts.ObjectStream, object.ObjectStream)
	}
	return object
}

type BeginSegment struct {
	Opts     metabase.BeginSegment
	ErrClass *errs.Class
	ErrText  string
}

func (step BeginSegment) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	err := db.BeginSegment(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

type CommitSegment struct {
	Opts     metabase.CommitSegment
	ErrClass *errs.Class
	ErrText  string
}

func (step CommitSegment) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	err := db.CommitSegment(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

type CommitInlineSegment struct {
	Opts     metabase.CommitInlineSegment
	ErrClass *errs.Class
	ErrText  string
}

func (step CommitInlineSegment) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	err := db.CommitInlineSegment(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

type UpdateObjectMetadata struct {
	Opts     metabase.UpdateObjectMetadata
	ErrClass *errs.Class
	ErrText  string
}

func (step UpdateObjectMetadata) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	err := db.UpdateObjectMetadata(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)
}

type GetObjectExactVersion struct {
	Opts     metabase.GetObjectExactVersion
	Result   metabase.Object
	ErrClass *errs.Class
	ErrText  string
}

func (step GetObjectExactVersion) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
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

func (step GetObjectLatestVersion) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	result, err := db.GetObjectLatestVersion(ctx, step.Opts)
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

func (step GetSegmentByPosition) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
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

func (step GetLatestObjectLastSegment) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
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

func (step GetSegmentByOffset) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	result, err := db.GetSegmentByOffset(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type ListSegments struct {
	Opts     metabase.ListSegments
	Result   metabase.ListSegmentsResult
	ErrClass *errs.Class
	ErrText  string
}

func (step ListSegments) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	result, err := db.ListSegments(ctx, step.Opts)
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

func (step DeleteObjectExactVersion) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	result, err := db.DeleteObjectExactVersion(ctx, step.Opts)
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

func (step DeleteObjectLatestVersion) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	result, err := db.DeleteObjectLatestVersion(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type DeleteObjectAllVersions struct {
	Opts     metabase.DeleteObjectAllVersions
	Result   metabase.DeleteObjectResult
	ErrClass *errs.Class
	ErrText  string
}

func (step DeleteObjectAllVersions) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	result, err := db.DeleteObjectAllVersions(ctx, step.Opts)
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

func (step DeleteObjectsAllVersions) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	result, err := db.DeleteObjectsAllVersions(ctx, step.Opts)
	checkError(t, err, step.ErrClass, step.ErrText)

	sortObjectsByKey(result.Objects)
	sortObjectsByKey(step.Result.Objects)

	diff := cmp.Diff(step.Result, result, cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

func checkError(t *testing.T, err error, errClass *errs.Class, errText string) {
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

func sortObjectsByKey(objects []metabase.Object) {
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].ObjectKey < objects[j].ObjectKey
	})
}

type DeleteAll struct{}

func (step DeleteAll) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	err := db.TestingDeleteAll(ctx)
	require.NoError(t, err)
}

type Verify metabase.RawState

func (step Verify) Check(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
	state, err := db.TestingGetState(ctx)
	require.NoError(t, err)

	diff := cmp.Diff(metabase.RawState(step), *state,
		cmpopts.EquateApproxTime(5*time.Second))
	require.Zero(t, diff)
}

type CreateTestObject struct {
	BeginObjectExactVersion *metabase.BeginObjectExactVersion
	CommitObject            *metabase.CommitObject
	// TODO add BeginSegment, CommitSegment
}

func (co CreateTestObject) Run(ctx *testcontext.Context, t *testing.T, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte) {
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

	CommitObject{
		Opts: coOpts,
	}.Check(ctx, t, db)
}
