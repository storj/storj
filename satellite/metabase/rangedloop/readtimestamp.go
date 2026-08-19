// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"errors"
	"slices"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/dbutil"
)

// ServesFixedReadTimestamp reports whether every metabase backend can read at a
// fixed timestamp. Postgres has no AS OF SYSTEM TIME, and the scan reads it live
// whatever is configured.
func ServesFixedReadTimestamp(impls map[string]dbutil.Implementation) bool {
	return CanReadSnapshot(true, false, impls)
}

// ErrNoSnapshot refuses a bloom filter run whose scan would read no snapshot.
var ErrNoSnapshot = errors.New("the segment scan does not read a snapshot: set ranged-loop.stale-interval on a metabase whose backends can serve it, or ranged-loop.allow-live-reads on a single-backend test or restored-backup metabase")

// CanReadSnapshot reports whether a scan reads one snapshot of the whole
// metabase. fixed says a read timestamp is set, and then every backend has to
// serve it. Otherwise live reads have to be allowed and no backend may be able
// to serve a timestamp at all, so that the scan is no worse than one: that is
// a single Postgres backend for testing or a restored backup, one snapshot by
// construction. A mixed metabase never qualifies for live reads, as the
// backends that serve the timestamp would read a snapshot the others do not,
// and bloom filters built from that miss the pieces of a concurrent
// server-side copy on the live-reading ones.
func CanReadSnapshot(fixed, allowLiveReads bool, impls map[string]dbutil.Implementation) bool {
	serving := 0
	for _, impl := range impls {
		if impl.AsOfSystemTime(time.Now()) != "" {
			serving++
		}
	}
	return len(impls) > 0 && ((fixed && serving == len(impls)) || (allowLiveReads && serving == 0))
}

// WarnLiveReads logs that the scan reads live, which ranged-loop.allow-live-reads
// permits where no backend could serve a fixed read timestamp, for testing and
// restored backups only; so that the requirement of a snapshot is not taken as
// an assurance those backends cannot give. A mixed metabase is refused by the
// scan itself, so nothing is logged for it here.
func WarnLiveReads(log *zap.Logger, impls map[string]dbutil.Implementation) {
	if len(impls) == 0 || !CanReadSnapshot(false, true, impls) {
		return
	}
	backends := make([]string, 0, len(impls))
	for label, impl := range impls {
		backends = append(backends, label+"="+impl.String())
	}
	log.Warn("no metabase backend can read at a fixed timestamp and live reads are allowed: the scan reads live and can miss the pieces of a concurrent server-side copy, which is acceptable for testing and restored backups only",
		zap.Strings("backends", backends))
}

// PublishingObserver is an observer that publishes what the scan produced, and
// that lets the run hold that publication back.
type PublishingObserver interface {
	// ProcessedSegments is the number of segments the scan has fed it.
	ProcessedSegments() uint64
	// SetPublishGuard registers a check that has to pass before publishing.
	SetPublishGuard(guard func(ctx context.Context) error)
}

// AddSegmentsCountChecks returns observers that check the scan against the
// number of segments the snapshot at readTimestamp holds: an appended
// SegmentsCountValidation reports a mismatch per source, and every observer
// that publishes what the scan produced is held back on the same count, so
// that a short scan leaves no generation behind. Without a read timestamp,
// or where a backend cannot count at one, the counts would describe no
// snapshot and the checks are left out.
func AddSegmentsCountChecks(log *zap.Logger, mb *metabase.DB, readTimestamp time.Time, observers []Observer) []Observer {
	if readTimestamp.IsZero() {
		return observers
	}
	validation := NewSegmentsCountValidation(log, mb, readTimestamp, 0)
	observers = append(slices.Clone(observers), validation)
	if !ServesFixedReadTimestamp(mb.Implementations()) {
		return observers
	}
	for _, observer := range observers {
		if publishing, ok := observer.(PublishingObserver); ok {
			publishing.SetPublishGuard(PinnedSegmentsCountGuard(validation.SegmentsCount, publishing.ProcessedSegments))
		}
	}
	return observers
}

// PinnedSegmentsCountGuard returns a check that compares the number of
// segments a scan processed with the number the snapshot holds, for a job to
// run before it publishes anything the scan produced. Both come from the
// SegmentsCountValidation of the same run, which counts the snapshot once.
//
// The validation reports the same mismatch itself, but only after the run:
// the ranged loop finishes its observers in order, so by the time it compares
// its counts the bloom filter observer has already uploaded its generation and
// pointed LATEST at it. This one runs while the job can still refuse to
// publish.
func PinnedSegmentsCountGuard(snapshot func() (count int64, counted bool), processed func() uint64) func(ctx context.Context) error {
	return func(context.Context) error {
		count, counted := snapshot()
		if !counted {
			// nothing to compare the scan against, and an unchecked
			// generation is worse than none
			return Error.New("the segments count of the snapshot is unknown")
		}
		if int64(processed()) != count {
			return Error.New("the scan processed %d segments, the snapshot holds %d", processed(), count)
		}
		return nil
	}
}
