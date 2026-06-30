// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/uuid"
)

// mirrorTimeout bounds how long a single mirrored write may run before it
// is abandoned, so a slow secondary frees its in-flight slot instead of starving
// the mirror queue.
//
// ponytail: fixed 60s, make it configurable if a backend needs a different bound.
const mirrorTimeout = 60 * time.Second

// mirrorConcurrency bounds the number of in-flight background writes; further
// writes are dropped (never blocking primary) while that many are outstanding.
const mirrorConcurrency = 512

// mirrorAdapter serves all reads and writes from the primary backend and
// additionally replays write traffic onto a secondary backend in the background.
// It is used to validate that a candidate backend can sustain production write
// load without disrupting the primary: the primary result is always authoritative
// and returned synchronously, while the mirrored secondary write is fire-and-forget
// (bounded, best-effort, dropped under saturation, errors only logged).
//
// The primary Adapter is embedded, so every read, lifecycle, transaction, and
// global method delegates to it unchanged; only the per-project write methods are
// overridden to also mirror.
//
// This is ONLY a load-testing tool, NOT a replication mechanism: mirrored writes
// are best-effort and lossy (dropped under saturation, timed out, ordered only
// per call), so the secondary is never a faithful copy of the primary and must
// not be relied upon as one. Use a real migration/replication process for that.
//
// Because each mirror runs in its own background goroutine, mirrored operations
// are not ordered relative to one another: a later call can reach the secondary
// before an earlier one finishes. For example BeginObject and the following
// CommitObject race on the mirror, so the commit may run before (or without) its
// pending object and fail. These failures are expected and only logged; the load
// they generate is still representative.
//
// Known gaps (acceptable for a load test, not for correctness):
//   - WithTx (server-side copy/move) runs on primary only and is not mirrored.
//   - Node aliases live on adapter[0]; mirrored segment writes carry the primary's
//     alias values, which may not resolve in the secondary. The write load is still
//     representative.
type mirrorAdapter struct {
	Adapter // primary

	secondary Adapter
	log       *zap.Logger

	bgctx  context.Context
	cancel context.CancelFunc
	sem    chan struct{} // bounds concurrent in-flight mirrored writes
	wg     sync.WaitGroup
}

var _ Adapter = (*mirrorAdapter)(nil)

// newMirrorAdapter builds a mirrorAdapter.
func newMirrorAdapter(log *zap.Logger, primary, secondary Adapter) *mirrorAdapter {
	bgctx, cancel := context.WithCancel(context.Background())
	return &mirrorAdapter{
		Adapter:   primary,
		secondary: secondary,
		log:       log,
		bgctx:     bgctx,
		cancel:    cancel,
		sem:       make(chan struct{}, mirrorConcurrency),
	}
}

// Name returns the name of the adapter.
func (r *mirrorAdapter) Name() string {
	return "mirror(" + r.Adapter.Name() + "->" + r.secondary.Name() + ")"
}

// Close stops accepting new mirrored writes, cancels any in flight, and waits for
// them to finish. It does not close the underlying adapters; those are owned and
// closed by the DB.
func (r *mirrorAdapter) Close() error {
	r.cancel()
	r.wg.Wait()
	return nil
}

// mirror runs fn against the secondary backend in the background. It returns
// immediately. If too many writes are already in flight the call is dropped so the
// primary path is never blocked.
func (r *mirrorAdapter) mirror(op string, fn func(ctx context.Context) error) {
	select {
	case r.sem <- struct{}{}:
	default:
		mon.Counter("metabase_mirror_dropped").Inc(1)
		return
	}
	r.wg.Go(func() {
		defer func() { <-r.sem }()

		ctx, cancel := context.WithTimeout(r.bgctx, mirrorTimeout)
		defer cancel()

		if err := fn(ctx); err != nil {
			mon.Counter("metabase_mirror_failed").Inc(1)
			r.log.Debug("mirrored write failed", zap.String("op", op), zap.Error(err))
		}
	})
}

//
// Object creation and commits.
//

func (r *mirrorAdapter) BeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion) (Object, error) {
	obj, err := r.Adapter.BeginObjectExactVersion(ctx, opts)
	r.mirror("BeginObjectExactVersion", func(ctx context.Context) error {
		_, err := r.secondary.BeginObjectExactVersion(ctx, opts)
		return err
	})
	return obj, err
}

func (r *mirrorAdapter) BeginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion) (Object, error) {
	obj, err := r.Adapter.BeginObjectNextVersion(ctx, opts)
	r.mirror("BeginObjectNextVersion", func(ctx context.Context) error {
		_, err := r.secondary.BeginObjectNextVersion(ctx, opts)
		return err
	})
	return obj, err
}

func (r *mirrorAdapter) CommitObject(ctx context.Context, opts CommitObject) (Object, error) {
	obj, err := r.Adapter.CommitObject(ctx, opts)
	r.mirror("CommitObject", func(ctx context.Context) error {
		_, err := r.secondary.CommitObject(ctx, opts)
		return err
	})
	return obj, err
}

func (r *mirrorAdapter) CommitInlineObject(ctx context.Context, opts CommitInlineObject) (Object, error) {
	obj, err := r.Adapter.CommitInlineObject(ctx, opts)
	r.mirror("CommitInlineObject", func(ctx context.Context) error {
		_, err := r.secondary.CommitInlineObject(ctx, opts)
		return err
	})
	return obj, err
}

func (r *mirrorAdapter) CommitPendingObjectSegment(ctx context.Context, opts CommitSegment, aliasPieces AliasPieces) error {
	err := r.Adapter.CommitPendingObjectSegment(ctx, opts, aliasPieces)
	r.mirror("CommitPendingObjectSegment", func(ctx context.Context) error {
		return r.secondary.CommitPendingObjectSegment(ctx, opts, aliasPieces)
	})
	return err
}

func (r *mirrorAdapter) CommitInlineSegment(ctx context.Context, opts CommitInlineSegment) error {
	err := r.Adapter.CommitInlineSegment(ctx, opts)
	r.mirror("CommitInlineSegment", func(ctx context.Context) error {
		return r.secondary.CommitInlineSegment(ctx, opts)
	})
	return err
}

//
// Object mutations.
//

func (r *mirrorAdapter) SetObjectExactVersionRetention(ctx context.Context, opts SetObjectExactVersionRetention) error {
	err := r.Adapter.SetObjectExactVersionRetention(ctx, opts)
	r.mirror("SetObjectExactVersionRetention", func(ctx context.Context) error {
		return r.secondary.SetObjectExactVersionRetention(ctx, opts)
	})
	return err
}

func (r *mirrorAdapter) SetObjectLastCommittedRetention(ctx context.Context, opts SetObjectLastCommittedRetention) error {
	err := r.Adapter.SetObjectLastCommittedRetention(ctx, opts)
	r.mirror("SetObjectLastCommittedRetention", func(ctx context.Context) error {
		return r.secondary.SetObjectLastCommittedRetention(ctx, opts)
	})
	return err
}

func (r *mirrorAdapter) SetObjectExactVersionLegalHold(ctx context.Context, opts SetObjectExactVersionLegalHold) error {
	err := r.Adapter.SetObjectExactVersionLegalHold(ctx, opts)
	r.mirror("SetObjectExactVersionLegalHold", func(ctx context.Context) error {
		return r.secondary.SetObjectExactVersionLegalHold(ctx, opts)
	})
	return err
}

func (r *mirrorAdapter) SetObjectLastCommittedLegalHold(ctx context.Context, opts SetObjectLastCommittedLegalHold) error {
	err := r.Adapter.SetObjectLastCommittedLegalHold(ctx, opts)
	r.mirror("SetObjectLastCommittedLegalHold", func(ctx context.Context) error {
		return r.secondary.SetObjectLastCommittedLegalHold(ctx, opts)
	})
	return err
}

func (r *mirrorAdapter) UpdateObjectLastCommittedMetadata(ctx context.Context, opts UpdateObjectLastCommittedMetadata) error {
	err := r.Adapter.UpdateObjectLastCommittedMetadata(ctx, opts)
	r.mirror("UpdateObjectLastCommittedMetadata", func(ctx context.Context) error {
		return r.secondary.UpdateObjectLastCommittedMetadata(ctx, opts)
	})
	return err
}

func (r *mirrorAdapter) UpdateSegmentPieces(ctx context.Context, opts UpdateSegmentPieces, oldPieces, newPieces AliasPieces) (AliasPieces, error) {
	result, err := r.Adapter.UpdateSegmentPieces(ctx, opts, oldPieces, newPieces)
	r.mirror("UpdateSegmentPieces", func(ctx context.Context) error {
		_, err := r.secondary.UpdateSegmentPieces(ctx, opts, oldPieces, newPieces)
		return err
	})
	return result, err
}

func (r *mirrorAdapter) BatchUpdateSegmentPieces(ctx context.Context, opts BatchUpdateSegmentPieces, newAliasPieces []AliasPieces) ([]bool, error) {
	results, err := r.Adapter.BatchUpdateSegmentPieces(ctx, opts, newAliasPieces)
	r.mirror("BatchUpdateSegmentPieces", func(ctx context.Context) error {
		_, err := r.secondary.BatchUpdateSegmentPieces(ctx, opts, newAliasPieces)
		return err
	})
	return results, err
}

//
// Deletes.
//

func (r *mirrorAdapter) DeleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (DeleteObjectResult, error) {
	result, err := r.Adapter.DeleteObjectExactVersion(ctx, opts)
	r.mirror("DeleteObjectExactVersion", func(ctx context.Context) error {
		_, err := r.secondary.DeleteObjectExactVersion(ctx, opts)
		return err
	})
	return result, err
}

func (r *mirrorAdapter) DeletePendingObject(ctx context.Context, opts DeletePendingObject) (DeleteObjectResult, error) {
	result, err := r.Adapter.DeletePendingObject(ctx, opts)
	r.mirror("DeletePendingObject", func(ctx context.Context) error {
		_, err := r.secondary.DeletePendingObject(ctx, opts)
		return err
	})
	return result, err
}

func (r *mirrorAdapter) DeleteObjectLastCommittedPlain(ctx context.Context, opts DeleteObjectLastCommitted) (DeleteObjectResult, error) {
	result, err := r.Adapter.DeleteObjectLastCommittedPlain(ctx, opts)
	r.mirror("DeleteObjectLastCommittedPlain", func(ctx context.Context) error {
		_, err := r.secondary.DeleteObjectLastCommittedPlain(ctx, opts)
		return err
	})
	return result, err
}

func (r *mirrorAdapter) DeleteObjectLastCommittedVersioned(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (DeleteObjectResult, error) {
	result, err := r.Adapter.DeleteObjectLastCommittedVersioned(ctx, opts, deleterMarkerStreamID)
	r.mirror("DeleteObjectLastCommittedVersioned", func(ctx context.Context) error {
		_, err := r.secondary.DeleteObjectLastCommittedVersioned(ctx, opts, deleterMarkerStreamID)
		return err
	})
	return result, err
}

func (r *mirrorAdapter) DeleteAllBucketObjects(ctx context.Context, opts DeleteAllBucketObjects) (int64, int64, error) {
	objects, segments, err := r.Adapter.DeleteAllBucketObjects(ctx, opts)
	r.mirror("DeleteAllBucketObjects", func(ctx context.Context) error {
		_, _, err := r.secondary.DeleteAllBucketObjects(ctx, opts)
		return err
	})
	return objects, segments, err
}

func (r *mirrorAdapter) UncoordinatedDeleteAllBucketObjects(ctx context.Context, opts UncoordinatedDeleteAllBucketObjects) (int64, int64, error) {
	objects, segments, err := r.Adapter.UncoordinatedDeleteAllBucketObjects(ctx, opts)
	r.mirror("UncoordinatedDeleteAllBucketObjects", func(ctx context.Context) error {
		_, _, err := r.secondary.UncoordinatedDeleteAllBucketObjects(ctx, opts)
		return err
	})
	return objects, segments, err
}
