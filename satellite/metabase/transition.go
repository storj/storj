// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"slices"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil"
)

// transitionAdapter routes metabase operations across two backends during a
// database-to-database transition. primary is the new DB; secondary is the old
// DB. New writes land in primary; existing data is read from whichever DB owns
// it, with primary taking precedence. It performs no data migration itself: the
// bulk relocation secondary→primary is handled by a separate process.
//
// Routing rules (see docs):
//   - point read: try primary, fall back to secondary on not-found;
//   - write to existing object: try primary, fall back to secondary;
//   - begin object (create): co-locate with where the location already lives,
//     new locations go to primary;
//   - delete: apply to both, so no stale copy survives a relocation window;
//   - list/iterate: query both and merge.
//
// NOTE: global, project-agnostic operations (loop, accounting tally, etc.) fan
// out over db.adapters directly and therefore already cover both backends; the
// transitionAdapter is only reached through DB.ChooseAdapter for per-project
// operations. The global methods are still implemented here (defensively) so
// the type satisfies Adapter.
type transitionAdapter struct {
	primary   Adapter
	secondary Adapter
}

var _ Adapter = (*transitionAdapter)(nil)

// newTransitionAdapter constructs a transitionAdapter from the new (primary) and
// old (secondary) backends.
func newTransitionAdapter(primary, secondary Adapter) *transitionAdapter {
	return &transitionAdapter{primary: primary, secondary: secondary}
}

// isNotFound reports whether err indicates the object/segment/pending object was
// absent, i.e. that a fall-back to the other backend is warranted.
func isNotFound(err error) bool {
	return ErrObjectNotFound.Has(err) ||
		ErrSegmentNotFound.Has(err) ||
		ErrPendingObjectMissing.Has(err)
}

// transitionReadFallback runs primaryFn and, only when it reports not-found, retries with
// secondaryFn.
func transitionReadFallback[T any](primaryFn, secondaryFn func() (T, error)) (T, error) {
	v, err := primaryFn()
	if err != nil && isNotFound(err) {
		return secondaryFn()
	}
	return v, err
}

// transitionWriteFallback runs primaryFn and, only when it reports not-found, retries with
// secondaryFn. Used for mutations of an existing object that lives in exactly
// one backend.
func transitionWriteFallback(primaryFn, secondaryFn func() error) error {
	err := primaryFn()
	if err != nil && isNotFound(err) {
		return secondaryFn()
	}
	return err
}

// homeForObject returns the backend that owns the given location: primary if it
// already holds a committed object there, otherwise secondary if it holds one,
// otherwise primary (a brand-new location goes to the new DB).
func (t *transitionAdapter) homeForObject(ctx context.Context, loc ObjectLocation) Adapter {
	if _, err := t.primary.GetObjectLastCommitted(ctx, GetObjectLastCommitted{ObjectLocation: loc}); err == nil {
		return t.primary
	}
	if _, err := t.secondary.GetObjectLastCommitted(ctx, GetObjectLastCommitted{ObjectLocation: loc}); err == nil {
		return t.secondary
	}
	return t.primary
}

// mergeDeleteResults combines two delete results.
func mergeDeleteResults(a, b DeleteObjectResult) DeleteObjectResult {
	return DeleteObjectResult{
		Removed:             append(append([]Object{}, a.Removed...), b.Removed...),
		Markers:             append(append([]Object{}, a.Markers...), b.Markers...),
		DeletedSegmentCount: a.DeletedSegmentCount + b.DeletedSegmentCount,
	}
}

// deleteBoth applies a delete to both backends and merges the results. A
// not-found from a single backend is tolerated; if both report not-found the
// not-found error is returned.
func deleteBoth(primaryFn, secondaryFn func() (DeleteObjectResult, error)) (DeleteObjectResult, error) {
	pr, perr := primaryFn()
	if perr != nil && !isNotFound(perr) {
		return DeleteObjectResult{}, perr
	}
	sr, serr := secondaryFn()
	if serr != nil && !isNotFound(serr) {
		return DeleteObjectResult{}, serr
	}
	if isNotFound(perr) && isNotFound(serr) {
		return DeleteObjectResult{}, perr
	}
	return mergeDeleteResults(pr, sr), nil
}

//
// Metadata / lifecycle.
//

// Name returns the name of the adapter.
func (t *transitionAdapter) Name() string {
	return "transition(" + t.primary.Name() + "<-" + t.secondary.Name() + ")"
}

// Implementation returns the implementation of the primary backend.
func (t *transitionAdapter) Implementation() dbutil.Implementation {
	return dbutil.Unknown
}

// Config returns the metabase configuration.
func (t *transitionAdapter) Config() *Config {
	return t.primary.Config()
}

// Now returns the current time according to the primary backend.
func (t *transitionAdapter) Now(ctx context.Context) (time.Time, error) {
	return t.primary.Now(ctx)
}

// Ping checks both backends.
func (t *transitionAdapter) Ping(ctx context.Context) error {
	return errs.Combine(t.primary.Ping(ctx), t.secondary.Ping(ctx))
}

// MigrateToLatest migrates both backends to the latest version.
func (t *transitionAdapter) MigrateToLatest(ctx context.Context) error {
	// Technically this is redundant because the adapters should be separately registered as well.
	return errs.Combine(t.primary.MigrateToLatest(ctx), t.secondary.MigrateToLatest(ctx))
}

// CheckVersion checks both backends are at the correct version.
func (t *transitionAdapter) CheckVersion(ctx context.Context) error {
	// Technically this is redundant because the adapters should be separately registered as well.
	return errs.Combine(t.primary.CheckVersion(ctx), t.secondary.CheckVersion(ctx))
}

// TestMigrateToLatest migrates both backends for test purposes.
func (t *transitionAdapter) TestMigrateToLatest(ctx context.Context) error {
	// Technically this is redundant because the adapters should be separately registered as well.
	return errs.Combine(t.primary.TestMigrateToLatest(ctx), t.secondary.TestMigrateToLatest(ctx))
}

// WithTx is not supported on the transition adapter and fails closed.
//
// A transaction cannot span two engines, and WithTx is not given the object
// location, so the composite cannot pick the owning backend. The db-level
// callers that open a transaction to mutate an existing object
// (DeleteObjectLastCommittedSuspended, FinishCopyObject, FinishMoveObject) must
// select the owning backend themselves before calling WithTx. Until that
// "Tier B" work is done, routing here would silently operate on the wrong
// backend, so we return an error instead of corrupting state.
func (t *transitionAdapter) WithTx(ctx context.Context, opts TransactionOptions, f func(context.Context, TransactionAdapter) error) error {
	return Error.New("WithTx is not supported on the transition adapter: the caller must select the owning backend (delete-suspended / finish-copy / finish-move)")
}

//
// Point reads — try primary, fall back to secondary.
//

func (t *transitionAdapter) GetObjectExactVersion(ctx context.Context, opts GetObjectExactVersion) (Object, error) {
	return transitionReadFallback(
		func() (Object, error) { return t.primary.GetObjectExactVersion(ctx, opts) },
		func() (Object, error) { return t.secondary.GetObjectExactVersion(ctx, opts) },
	)
}

func (t *transitionAdapter) GetObjectLastCommitted(ctx context.Context, opts GetObjectLastCommitted) (Object, error) {
	return transitionReadFallback(
		func() (Object, error) { return t.primary.GetObjectLastCommitted(ctx, opts) },
		func() (Object, error) { return t.secondary.GetObjectLastCommitted(ctx, opts) },
	)
}

func (t *transitionAdapter) GetObjectExactVersionRetention(ctx context.Context, opts GetObjectExactVersionRetention) (Retention, error) {
	return transitionReadFallback(
		func() (Retention, error) { return t.primary.GetObjectExactVersionRetention(ctx, opts) },
		func() (Retention, error) { return t.secondary.GetObjectExactVersionRetention(ctx, opts) },
	)
}

func (t *transitionAdapter) GetObjectLastCommittedRetention(ctx context.Context, opts GetObjectLastCommittedRetention) (Retention, error) {
	return transitionReadFallback(
		func() (Retention, error) { return t.primary.GetObjectLastCommittedRetention(ctx, opts) },
		func() (Retention, error) { return t.secondary.GetObjectLastCommittedRetention(ctx, opts) },
	)
}

func (t *transitionAdapter) GetObjectExactVersionLegalHold(ctx context.Context, opts GetObjectExactVersionLegalHold) (bool, error) {
	return transitionReadFallback(
		func() (bool, error) { return t.primary.GetObjectExactVersionLegalHold(ctx, opts) },
		func() (bool, error) { return t.secondary.GetObjectExactVersionLegalHold(ctx, opts) },
	)
}

func (t *transitionAdapter) GetObjectLastCommittedLegalHold(ctx context.Context, opts GetObjectLastCommittedLegalHold) (bool, error) {
	return transitionReadFallback(
		func() (bool, error) { return t.primary.GetObjectLastCommittedLegalHold(ctx, opts) },
		func() (bool, error) { return t.secondary.GetObjectLastCommittedLegalHold(ctx, opts) },
	)
}

func (t *transitionAdapter) GetLatestObjectLastSegment(ctx context.Context, opts GetLatestObjectLastSegment) (Segment, error) {
	return transitionReadFallback(
		func() (Segment, error) { return t.primary.GetLatestObjectLastSegment(ctx, opts) },
		func() (Segment, error) { return t.secondary.GetLatestObjectLastSegment(ctx, opts) },
	)
}

func (t *transitionAdapter) GetSegmentByPosition(ctx context.Context, opts GetSegmentByPosition) (Segment, AliasPieces, error) {
	segment, aliasPieces, err := t.primary.GetSegmentByPosition(ctx, opts)
	if err != nil && isNotFound(err) {
		return t.secondary.GetSegmentByPosition(ctx, opts)
	}
	return segment, aliasPieces, err
}

func (t *transitionAdapter) GetSegmentByPositionForAudit(ctx context.Context, opts GetSegmentByPosition) (SegmentForAudit, AliasPieces, error) {
	segment, aliasPieces, err := t.primary.GetSegmentByPositionForAudit(ctx, opts)
	if err != nil && isNotFound(err) {
		return t.secondary.GetSegmentByPositionForAudit(ctx, opts)
	}
	return segment, aliasPieces, err
}

func (t *transitionAdapter) GetSegmentByPositionForRepair(ctx context.Context, opts GetSegmentByPosition) (SegmentForRepair, AliasPieces, error) {
	segment, aliasPieces, err := t.primary.GetSegmentByPositionForRepair(ctx, opts)
	if err != nil && isNotFound(err) {
		return t.secondary.GetSegmentByPositionForRepair(ctx, opts)
	}
	return segment, aliasPieces, err
}

func (t *transitionAdapter) GetSegmentsByPosition(ctx context.Context, opts GetSegmentsByPosition) (map[SegmentPositionKey]Segment, map[SegmentPositionKey]AliasPieces, error) {
	segments, aliasPiecesMap, err := t.primary.GetSegmentsByPosition(ctx, opts)
	if (err != nil && isNotFound(err)) || (err == nil && len(segments) == 0) {
		return t.secondary.GetSegmentsByPosition(ctx, opts)
	}
	return segments, aliasPiecesMap, err
}

func (t *transitionAdapter) GetSegmentPositionsAndKeys(ctx context.Context, streamID uuid.UUID) ([]EncryptedKeyAndNonce, error) {
	res, err := t.primary.GetSegmentPositionsAndKeys(ctx, streamID)
	if (err != nil && isNotFound(err)) || (err == nil && len(res) == 0) {
		return t.secondary.GetSegmentPositionsAndKeys(ctx, streamID)
	}
	return res, err
}

func (t *transitionAdapter) GetStreamPieceCountByAlias(ctx context.Context, opts GetStreamPieceCountByNodeID) (map[NodeAlias]int64, error) {
	res, err := t.primary.GetStreamPieceCountByAlias(ctx, opts)
	if (err != nil && isNotFound(err)) || (err == nil && len(res) == 0) {
		return t.secondary.GetStreamPieceCountByAlias(ctx, opts)
	}
	return res, err
}

func (t *transitionAdapter) CheckSegmentPiecesAlteration(ctx context.Context, streamID uuid.UUID, position SegmentPosition, aliasPieces AliasPieces) (bool, error) {
	return transitionReadFallback(
		func() (bool, error) {
			return t.primary.CheckSegmentPiecesAlteration(ctx, streamID, position, aliasPieces)
		},
		func() (bool, error) {
			return t.secondary.CheckSegmentPiecesAlteration(ctx, streamID, position, aliasPieces)
		},
	)
}

func (t *transitionAdapter) GetPendingObjectMetadata(ctx context.Context, opts GetPendingObjectMetadata) (GetPendingObjectMetadataResult, error) {
	return transitionReadFallback(
		func() (GetPendingObjectMetadataResult, error) { return t.primary.GetPendingObjectMetadata(ctx, opts) },
		func() (GetPendingObjectMetadataResult, error) {
			return t.secondary.GetPendingObjectMetadata(ctx, opts)
		},
	)
}

// PendingObjectExists is true if the pending object exists in either backend.
func (t *transitionAdapter) PendingObjectExists(ctx context.Context, opts BeginSegment) (bool, error) {
	exists, err := t.primary.PendingObjectExists(ctx, opts)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}
	return t.secondary.PendingObjectExists(ctx, opts)
}

// BucketEmpty is true only if the bucket is empty in both backends.
func (t *transitionAdapter) BucketEmpty(ctx context.Context, opts BucketEmpty) (bool, error) {
	empty, err := t.primary.BucketEmpty(ctx, opts)
	if err != nil {
		return false, err
	}
	if !empty {
		return false, nil
	}
	return t.secondary.BucketEmpty(ctx, opts)
}

//
// Object creation — co-locate with the owning backend.
//

func (t *transitionAdapter) BeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion) (Object, error) {
	return t.homeForObject(ctx, opts.Location()).BeginObjectExactVersion(ctx, opts)
}

func (t *transitionAdapter) BeginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion) (Object, error) {
	return t.homeForObject(ctx, opts.Location()).BeginObjectNextVersion(ctx, opts)
}

//
// Object/segment commits and mutations — operate where the object lives.
//

func (t *transitionAdapter) CommitObject(ctx context.Context, opts CommitObject) (Object, error) {
	// CommitObject must run on the backend that holds the pending object (placed
	// by BeginObject via home(K)). A primary-first fallback is unsafe: commitObject
	// does not return not-found when the pending object is absent — it commits a
	// fresh object from the option values — so a fallback would never trigger and
	// would commit an empty object into primary, shadowing the real data (and
	// orphaning the segments) in secondary. Route via the same home(K).
	return t.homeForObject(ctx, opts.Location()).CommitObject(ctx, opts)
}

func (t *transitionAdapter) CommitInlineObject(ctx context.Context, opts CommitInlineObject) (Object, error) {
	// CommitInlineObject creates a committed object in one shot (no prior pending
	// object to locate), so it must be co-located via home(K) like BeginObject.
	// A plain primary-first fallback would always succeed on primary and could
	// create a second committed object for a location owned by secondary.
	return t.homeForObject(ctx, opts.Location()).CommitInlineObject(ctx, opts)
}

func (t *transitionAdapter) CommitPendingObjectSegment(ctx context.Context, opts CommitSegment, aliasPieces AliasPieces) error {
	return transitionWriteFallback(
		func() error { return t.primary.CommitPendingObjectSegment(ctx, opts, aliasPieces) },
		func() error { return t.secondary.CommitPendingObjectSegment(ctx, opts, aliasPieces) },
	)
}

func (t *transitionAdapter) CommitInlineSegment(ctx context.Context, opts CommitInlineSegment) error {
	return transitionWriteFallback(
		func() error { return t.primary.CommitInlineSegment(ctx, opts) },
		func() error { return t.secondary.CommitInlineSegment(ctx, opts) },
	)
}

func (t *transitionAdapter) SetObjectExactVersionRetention(ctx context.Context, opts SetObjectExactVersionRetention) error {
	return transitionWriteFallback(
		func() error { return t.primary.SetObjectExactVersionRetention(ctx, opts) },
		func() error { return t.secondary.SetObjectExactVersionRetention(ctx, opts) },
	)
}

func (t *transitionAdapter) SetObjectLastCommittedRetention(ctx context.Context, opts SetObjectLastCommittedRetention) error {
	return transitionWriteFallback(
		func() error { return t.primary.SetObjectLastCommittedRetention(ctx, opts) },
		func() error { return t.secondary.SetObjectLastCommittedRetention(ctx, opts) },
	)
}

func (t *transitionAdapter) SetObjectExactVersionLegalHold(ctx context.Context, opts SetObjectExactVersionLegalHold) error {
	return transitionWriteFallback(
		func() error { return t.primary.SetObjectExactVersionLegalHold(ctx, opts) },
		func() error { return t.secondary.SetObjectExactVersionLegalHold(ctx, opts) },
	)
}

func (t *transitionAdapter) SetObjectLastCommittedLegalHold(ctx context.Context, opts SetObjectLastCommittedLegalHold) error {
	return transitionWriteFallback(
		func() error { return t.primary.SetObjectLastCommittedLegalHold(ctx, opts) },
		func() error { return t.secondary.SetObjectLastCommittedLegalHold(ctx, opts) },
	)
}

func (t *transitionAdapter) UpdateObjectLastCommittedMetadata(ctx context.Context, opts UpdateObjectLastCommittedMetadata) error {
	return transitionWriteFallback(
		func() error { return t.primary.UpdateObjectLastCommittedMetadata(ctx, opts) },
		func() error { return t.secondary.UpdateObjectLastCommittedMetadata(ctx, opts) },
	)
}

func (t *transitionAdapter) UpdateSegmentPieces(ctx context.Context, opts UpdateSegmentPieces, oldPieces, newPieces AliasPieces) (AliasPieces, error) {
	return transitionReadFallback(
		func() (AliasPieces, error) { return t.primary.UpdateSegmentPieces(ctx, opts, oldPieces, newPieces) },
		func() (AliasPieces, error) {
			return t.secondary.UpdateSegmentPieces(ctx, opts, oldPieces, newPieces)
		},
	)
}

func (t *transitionAdapter) BatchUpdateSegmentPieces(ctx context.Context, opts BatchUpdateSegmentPieces, newAliasPieces []AliasPieces) ([]bool, error) {
	return transitionReadFallback(
		func() ([]bool, error) { return t.primary.BatchUpdateSegmentPieces(ctx, opts, newAliasPieces) },
		func() ([]bool, error) { return t.secondary.BatchUpdateSegmentPieces(ctx, opts, newAliasPieces) },
	)
}

//
// Deletes — apply to both backends.
//

func (t *transitionAdapter) DeleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (DeleteObjectResult, error) {
	return deleteBoth(
		func() (DeleteObjectResult, error) { return t.primary.DeleteObjectExactVersion(ctx, opts) },
		func() (DeleteObjectResult, error) { return t.secondary.DeleteObjectExactVersion(ctx, opts) },
	)
}

func (t *transitionAdapter) DeletePendingObject(ctx context.Context, opts DeletePendingObject) (DeleteObjectResult, error) {
	return deleteBoth(
		func() (DeleteObjectResult, error) { return t.primary.DeletePendingObject(ctx, opts) },
		func() (DeleteObjectResult, error) { return t.secondary.DeletePendingObject(ctx, opts) },
	)
}

func (t *transitionAdapter) DeleteObjectLastCommittedPlain(ctx context.Context, opts DeleteObjectLastCommitted) (DeleteObjectResult, error) {
	return deleteBoth(
		func() (DeleteObjectResult, error) { return t.primary.DeleteObjectLastCommittedPlain(ctx, opts) },
		func() (DeleteObjectResult, error) { return t.secondary.DeleteObjectLastCommittedPlain(ctx, opts) },
	)
}

func (t *transitionAdapter) DeleteObjectLastCommittedVersioned(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (DeleteObjectResult, error) {
	// A versioned "delete" inserts a delete marker — it is a write, not a
	// removal, so it must go to the single backend that owns the location.
	// Applying it to both (like deleteBoth) would create a phantom delete marker
	// in the backend that never held the object.
	return t.homeForObject(ctx, opts.ObjectLocation).DeleteObjectLastCommittedVersioned(ctx, opts, deleterMarkerStreamID)
}

// DeleteAllBucketObjects deletes from both backends and sums the counts.
func (t *transitionAdapter) DeleteAllBucketObjects(ctx context.Context, opts DeleteAllBucketObjects) (int64, int64, error) {
	po, ps, err := t.primary.DeleteAllBucketObjects(ctx, opts)
	if err != nil {
		return 0, 0, err
	}
	so, ss, err := t.secondary.DeleteAllBucketObjects(ctx, opts)
	if err != nil {
		return 0, 0, err
	}
	return po + so, ps + ss, nil
}

// UncoordinatedDeleteAllBucketObjects deletes from both backends and sums the counts.
func (t *transitionAdapter) UncoordinatedDeleteAllBucketObjects(ctx context.Context, opts UncoordinatedDeleteAllBucketObjects) (int64, int64, error) {
	po, ps, err := t.primary.UncoordinatedDeleteAllBucketObjects(ctx, opts)
	if err != nil {
		return 0, 0, err
	}
	so, ss, err := t.secondary.UncoordinatedDeleteAllBucketObjects(ctx, opts)
	if err != nil {
		return 0, 0, err
	}
	return po + so, ps + ss, nil
}

//
// Per-project listings — query both and merge.
//

func (t *transitionAdapter) ListObjects(ctx context.Context, opts ListObjects) (ListObjectsResult, error) {
	pr, err := t.primary.ListObjects(ctx, opts)
	if err != nil {
		return ListObjectsResult{}, err
	}
	sr, err := t.secondary.ListObjects(ctx, opts)
	if err != nil {
		return ListObjectsResult{}, err
	}

	objects, truncated := mergeObjectEntries(pr.Objects, sr.Objects, opts.VersionAscending(), opts.Limit)
	return ListObjectsResult{
		Objects: objects,
		More:    pr.More || sr.More || truncated,
	}, nil
}

func (t *transitionAdapter) ListSegments(ctx context.Context, opts ListSegments, aliasCache *NodeAliasCache) (ListSegmentsResult, error) {
	pr, err := t.primary.ListSegments(ctx, opts, aliasCache)
	if err != nil {
		return ListSegmentsResult{}, err
	}
	sr, err := t.secondary.ListSegments(ctx, opts, aliasCache)
	if err != nil {
		return ListSegmentsResult{}, err
	}

	segments := mergeSegments(pr.Segments, sr.Segments)
	return ListSegmentsResult{
		Segments: segments,
		More:     pr.More || sr.More,
	}, nil
}

func (t *transitionAdapter) ListStreamPositions(ctx context.Context, opts ListStreamPositions) (ListStreamPositionsResult, error) {
	pr, err := t.primary.ListStreamPositions(ctx, opts)
	if err != nil {
		return ListStreamPositionsResult{}, err
	}
	sr, err := t.secondary.ListStreamPositions(ctx, opts)
	if err != nil {
		return ListStreamPositionsResult{}, err
	}

	segments := mergeStreamPositions(pr.Segments, sr.Segments)
	return ListStreamPositionsResult{
		Segments: segments,
		More:     pr.More || sr.More,
	}, nil
}

// ObjectIterator returns an iterator that merges the ordered streams from both backends.
func (t *transitionAdapter) ObjectIterator(ctx context.Context, opts ObjectIteratorOptions) (ObjectIterator, error) {
	primary, err := t.primary.ObjectIterator(ctx, opts)
	if err != nil {
		return nil, err
	}
	secondary, err := t.secondary.ObjectIterator(ctx, opts)
	if err != nil {
		_ = primary.Close()
		return nil, err
	}
	return &mergingObjectIterator{
		primary:          primary,
		secondary:        secondary,
		versionAscending: opts.Mode != ObjectIteratorModeAllVersionsDescending,
	}, nil
}

//
// Global / project-agnostic operations.
//
// These fan out over db.adapters directly in normal use, so the implementations
// below are defensive: they cover both backends in case the method is ever
// reached through the transition adapter.
//

func (t *transitionAdapter) IterateLoopSegments(ctx context.Context, aliasCache *NodeAliasCache, opts IterateLoopSegments, fn func(context.Context, LoopSegmentsIterator) error) error {
	if err := t.primary.IterateLoopSegments(ctx, aliasCache, opts, fn); err != nil {
		return err
	}
	return t.secondary.IterateLoopSegments(ctx, aliasCache, opts, fn)
}

func (t *transitionAdapter) ListVerifySegments(ctx context.Context, opts ListVerifySegments) ([]VerifySegment, error) {
	pr, err := t.primary.ListVerifySegments(ctx, opts)
	if err != nil {
		return nil, err
	}
	sr, err := t.secondary.ListVerifySegments(ctx, opts)
	if err != nil {
		return nil, err
	}
	return append(pr, sr...), nil
}

func (t *transitionAdapter) ListBucketStreamIDs(ctx context.Context, opts ListBucketStreamIDs, process func(ctx context.Context, streamIDs []uuid.UUID) error) error {
	if err := t.primary.ListBucketStreamIDs(ctx, opts, process); err != nil {
		return err
	}
	return t.secondary.ListBucketStreamIDs(ctx, opts, process)
}

func (t *transitionAdapter) IterateExpiredObjects(ctx context.Context, opts DeleteExpiredObjects, process func(context.Context, []ObjectStream) error) error {
	if err := t.primary.IterateExpiredObjects(ctx, opts, process); err != nil {
		return err
	}
	return t.secondary.IterateExpiredObjects(ctx, opts, process)
}

func (t *transitionAdapter) IterateZombieObjects(ctx context.Context, opts DeleteZombieObjects, process func(context.Context, []ObjectStream) error) error {
	if err := t.primary.IterateZombieObjects(ctx, opts, process); err != nil {
		return err
	}
	return t.secondary.IterateZombieObjects(ctx, opts, process)
}

func (t *transitionAdapter) DeleteObjectsAndSegmentsNoVerify(ctx context.Context, opts DeleteObjectsAndSegmentsNoVerify) (int64, int64, error) {
	po, ps, err := t.primary.DeleteObjectsAndSegmentsNoVerify(ctx, opts)
	if err != nil {
		return 0, 0, err
	}
	so, ss, err := t.secondary.DeleteObjectsAndSegmentsNoVerify(ctx, opts)
	if err != nil {
		return 0, 0, err
	}
	return po + so, ps + ss, nil
}

func (t *transitionAdapter) DeleteInactiveObjectsAndSegments(ctx context.Context, opts DeleteInactiveObjectsAndSegments) (int64, int64, error) {
	po, ps, err := t.primary.DeleteInactiveObjectsAndSegments(ctx, opts)
	if err != nil {
		return 0, 0, err
	}
	so, ss, err := t.secondary.DeleteInactiveObjectsAndSegments(ctx, opts)
	if err != nil {
		return 0, 0, err
	}
	return po + so, ps + ss, nil
}

func (t *transitionAdapter) CollectBucketTallies(ctx context.Context, opts CollectBucketTallies) ([]BucketTally, error) {
	pr, err := t.primary.CollectBucketTallies(ctx, opts)
	if err != nil {
		return nil, err
	}
	sr, err := t.secondary.CollectBucketTallies(ctx, opts)
	if err != nil {
		return nil, err
	}
	return append(pr, sr...), nil
}

func (t *transitionAdapter) CountSegments(ctx context.Context, checkTimestamp time.Time) (int64, error) {
	pc, err := t.primary.CountSegments(ctx, checkTimestamp)
	if err != nil {
		return 0, err
	}
	sc, err := t.secondary.CountSegments(ctx, checkTimestamp)
	if err != nil {
		return 0, err
	}
	return pc + sc, nil
}

// GetTableStats returns the primary backend's table stats.
//
// TODO(transition): combine stats across backends if this is ever needed via
// the transition path.
func (t *transitionAdapter) GetTableStats(ctx context.Context, opts GetTableStats) (TableStats, error) {
	return t.primary.GetTableStats(ctx, opts)
}

func (t *transitionAdapter) UpdateTableStats(ctx context.Context) error {
	return errs.Combine(t.primary.UpdateTableStats(ctx), t.secondary.UpdateTableStats(ctx))
}

//
// Node aliases — single shared backend (primary).
//

func (t *transitionAdapter) EnsureNodeAliases(ctx context.Context, opts EnsureNodeAliases) error {
	return t.primary.EnsureNodeAliases(ctx, opts)
}

func (t *transitionAdapter) ListNodeAliases(ctx context.Context) ([]NodeAliasEntry, error) {
	return t.primary.ListNodeAliases(ctx)
}

func (t *transitionAdapter) GetNodeAliasEntries(ctx context.Context, opts GetNodeAliasEntries) ([]NodeAliasEntry, error) {
	return t.primary.GetNodeAliasEntries(ctx, opts)
}

//
// copyObjectAdapter — not supported on the transition adapter.
//
// These are the unexported building blocks of server-side copy/move. They run
// inside a single-backend transaction (see WithTx) and are driven by db-level
// code that must select the owning backend itself. Reaching them through the
// transition adapter means the caller skipped that selection, so they fail
// closed rather than guess a backend.

// errTransitionInternal is returned by internal adapter methods that must not be
// invoked on the transition adapter directly.
func errTransitionInternal(method string) error {
	return Error.New("%s must not be called on the transition adapter: the caller must select the owning backend", method)
}

func (t *transitionAdapter) getSegmentsForCopy(ctx context.Context, object Object) (transposedSegmentList, error) {
	return transposedSegmentList{}, errTransitionInternal("getSegmentsForCopy")
}

func (t *transitionAdapter) getObjectNonPendingExactVersion(ctx context.Context, opts FinishCopyObject) (Object, error) {
	return Object{}, errTransitionInternal("getObjectNonPendingExactVersion")
}

func (t *transitionAdapter) finalizeSegmentsCopy(ctx context.Context, opts FinishCopyObject, newSegments transposedSegmentList) error {
	return errTransitionInternal("finalizeSegmentsCopy")
}

func (t *transitionAdapter) insertPendingCopyObject(ctx context.Context, opts FinishCopyObject, sourceObject Object, encryptedUserData EncryptedUserData) (Object, error) {
	return Object{}, errTransitionInternal("insertPendingCopyObject")
}

func (t *transitionAdapter) deleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (DeleteObjectResult, error) {
	return DeleteObjectResult{}, errTransitionInternal("deleteObjectExactVersion")
}

//
// Testing helpers.
//

func (t *transitionAdapter) TestingBatchInsertSegments(ctx context.Context, aliasCache *NodeAliasCache, segments []RawSegment) error {
	return t.primary.TestingBatchInsertSegments(ctx, aliasCache, segments)
}

func (t *transitionAdapter) TestingBatchInsertObjects(ctx context.Context, objects []RawObject) error {
	return t.primary.TestingBatchInsertObjects(ctx, objects)
}

func (t *transitionAdapter) TestingGetAllObjects(ctx context.Context) ([]RawObject, error) {
	pr, err := t.primary.TestingGetAllObjects(ctx)
	if err != nil {
		return nil, err
	}
	sr, err := t.secondary.TestingGetAllObjects(ctx)
	if err != nil {
		return nil, err
	}
	return append(pr, sr...), nil
}

func (t *transitionAdapter) TestingGetAllSegments(ctx context.Context, aliasCache *NodeAliasCache) ([]RawSegment, error) {
	pr, err := t.primary.TestingGetAllSegments(ctx, aliasCache)
	if err != nil {
		return nil, err
	}
	sr, err := t.secondary.TestingGetAllSegments(ctx, aliasCache)
	if err != nil {
		return nil, err
	}
	return append(pr, sr...), nil
}

func (t *transitionAdapter) TestingDeleteAll(ctx context.Context) error {
	return errs.Combine(t.primary.TestingDeleteAll(ctx), t.secondary.TestingDeleteAll(ctx))
}

func (t *transitionAdapter) TestingSetObjectVersion(ctx context.Context, object ObjectStream, randomVersion Version) (int64, error) {
	pr, err := t.primary.TestingSetObjectVersion(ctx, object, randomVersion)
	if err != nil {
		return 0, err
	}
	sr, err := t.secondary.TestingSetObjectVersion(ctx, object, randomVersion)
	if err != nil {
		return 0, err
	}
	return pr + sr, nil
}

func (t *transitionAdapter) TestingSetPlacementAllSegments(ctx context.Context, placement storj.PlacementConstraint) error {
	return errs.Combine(
		t.primary.TestingSetPlacementAllSegments(ctx, placement),
		t.secondary.TestingSetPlacementAllSegments(ctx, placement),
	)
}

func (t *transitionAdapter) TestingSetObjectCreatedAt(ctx context.Context, object ObjectStream, createdAt time.Time) (int64, error) {
	pr, err := t.primary.TestingSetObjectCreatedAt(ctx, object, createdAt)
	if err != nil {
		return 0, err
	}
	sr, err := t.secondary.TestingSetObjectCreatedAt(ctx, object, createdAt)
	if err != nil {
		return 0, err
	}
	return pr + sr, nil
}

//
// Merge helpers.
//

// mergeObjectEntries merges two listings of object entries, preferring primary
// on duplicate keys, sorting by the natural object order, and truncating to
// limit (0 means no limit). truncated reports whether entries were dropped.
func mergeObjectEntries(primary, secondary []ObjectEntry, versionAscending bool, limit int) (out []ObjectEntry, truncated bool) {
	out = make([]ObjectEntry, 0, len(primary)+len(secondary))
	out = append(out, primary...)

	// Dedup on (key, version) — not key alone — so that AllVersions listings,
	// where one key legitimately has multiple versions, are not collapsed. The
	// same (key, version) appearing in both backends is a relocation-window
	// duplicate; primary wins.
	//
	// Limitation: when a single key's versions are genuinely split across both
	// backends (only possible with versioned objects, which are outside the
	// transition's unversioned scope), the IsLatest flag is not reconciled and
	// more than one entry for that key may report IsLatest=true.
	seen := make(map[objectEntryKey]struct{}, len(primary))
	for _, e := range primary {
		seen[objectEntryKey{e.ObjectKey, e.Version}] = struct{}{}
	}
	for _, e := range secondary {
		if _, ok := seen[objectEntryKey{e.ObjectKey, e.Version}]; ok {
			continue
		}
		out = append(out, e)
	}

	slices.SortFunc(out, func(a, b ObjectEntry) int {
		switch {
		case objectEntryLess(a, b, versionAscending):
			return -1
		case objectEntryLess(b, a, versionAscending):
			return 1
		default:
			return 0
		}
	})

	if limit > 0 && len(out) > limit {
		out = out[:limit]
		truncated = true
	}
	return out, truncated
}

// objectEntryKey identifies a logical object version for cross-backend dedup.
type objectEntryKey struct {
	key     ObjectKey
	version Version
}

// objectEntryLess orders two entries the way the backend iterators do: key
// ascending, then version descending (the default) or ascending (pending and
// unversioned listings). The ordering must match opts so the merged stream
// stays globally sorted for downstream prefix/limit/cursor handling.
func objectEntryLess(a, b ObjectEntry, versionAscending bool) bool {
	sa := ObjectStream{ObjectKey: a.ObjectKey, Version: a.Version, StreamID: a.StreamID}
	sb := ObjectStream{ObjectKey: b.ObjectKey, Version: b.Version, StreamID: b.StreamID}
	if versionAscending {
		return sa.LessVersionAsc(sb)
	}
	return sa.Less(sb)
}

// mergeSegments merges segments from both backends, deduping by position
// (preferring primary) and sorting by position.
func mergeSegments(primary, secondary []Segment) []Segment {
	out := make([]Segment, 0, len(primary)+len(secondary))
	out = append(out, primary...)

	seen := make(map[SegmentPosition]struct{}, len(primary))
	for _, s := range primary {
		seen[s.Position] = struct{}{}
	}
	for _, s := range secondary {
		if _, ok := seen[s.Position]; ok {
			continue
		}
		out = append(out, s)
	}

	slices.SortFunc(out, func(a, b Segment) int {
		ae, be := a.Position.Encode(), b.Position.Encode()
		switch {
		case ae < be:
			return -1
		case ae > be:
			return 1
		default:
			return 0
		}
	})
	return out
}

// mergeStreamPositions merges segment position infos from both backends, deduping
// by position (preferring primary) and sorting by position.
func mergeStreamPositions(primary, secondary []SegmentPositionInfo) []SegmentPositionInfo {
	out := make([]SegmentPositionInfo, 0, len(primary)+len(secondary))
	out = append(out, primary...)

	seen := make(map[SegmentPosition]struct{}, len(primary))
	for _, s := range primary {
		seen[s.Position] = struct{}{}
	}
	for _, s := range secondary {
		if _, ok := seen[s.Position]; ok {
			continue
		}
		out = append(out, s)
	}

	slices.SortFunc(out, func(a, b SegmentPositionInfo) int {
		ae, be := a.Position.Encode(), b.Position.Encode()
		switch {
		case ae < be:
			return -1
		case ae > be:
			return 1
		default:
			return 0
		}
	})
	return out
}

// mergingObjectIterator merges two ordered ObjectIterators, emitting entries in
// the natural object order and preferring primary entries on duplicate keys.
type mergingObjectIterator struct {
	primary, secondary ObjectIterator

	// versionAscending selects the ordering to match the underlying iterators'
	// mode (key ASC, then version ASC for pending/ascending modes, else DESC).
	versionAscending bool

	primaryEntry   ObjectEntry
	secondaryEntry ObjectEntry
	primaryHas     bool
	secondaryHas   bool
	primaryDone    bool
	secondaryDone  bool
}

// fill ensures both lookahead buffers are populated where possible.
func (m *mergingObjectIterator) fill(ctx context.Context) error {
	if !m.primaryDone && !m.primaryHas {
		ok, err := m.primary.Next(ctx, &m.primaryEntry)
		if err != nil {
			return err
		}
		if ok {
			m.primaryHas = true
		} else {
			m.primaryDone = true
		}
	}
	if !m.secondaryDone && !m.secondaryHas {
		ok, err := m.secondary.Next(ctx, &m.secondaryEntry)
		if err != nil {
			return err
		}
		if ok {
			m.secondaryHas = true
		} else {
			m.secondaryDone = true
		}
	}
	return nil
}

// Next advances to the next merged entry.
func (m *mergingObjectIterator) Next(ctx context.Context, dst *ObjectEntry) (bool, error) {
	if err := m.fill(ctx); err != nil {
		return false, err
	}

	// Same logical version present in both (relocation window): primary wins.
	// Dedup on (key, version) so distinct versions of a key in AllVersions
	// listings are preserved. Handle this before the ordering comparison,
	// because the comparator tie-breaks on StreamID and could otherwise emit the
	// secondary entry first.
	if m.primaryHas && m.secondaryHas &&
		m.primaryEntry.ObjectKey == m.secondaryEntry.ObjectKey &&
		m.primaryEntry.Version == m.secondaryEntry.Version {
		*dst = m.primaryEntry
		m.primaryHas = false
		m.secondaryHas = false
		return true, nil
	}

	switch {
	case !m.primaryHas && !m.secondaryHas:
		return false, nil

	case m.primaryHas && (!m.secondaryHas || !objectEntryLess(m.secondaryEntry, m.primaryEntry, m.versionAscending)):
		// primary is the smaller (or equal) entry in the requested ordering.
		*dst = m.primaryEntry
		m.primaryHas = false
		return true, nil

	default:
		// secondary is strictly smaller.
		*dst = m.secondaryEntry
		m.secondaryHas = false
		return true, nil
	}
}

// Close releases both iterators.
func (m *mergingObjectIterator) Close() error {
	return errs.Combine(m.primary.Close(), m.secondary.Close())
}
