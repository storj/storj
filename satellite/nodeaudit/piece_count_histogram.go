// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeaudit

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/overlay"
)

var monPieceCountHistogram = monkit.Package()

// PieceCountBucket identifies one bucket of the piece count histogram.
type PieceCountBucket struct {
	// Placement is the placement constraint of the segment.
	Placement storj.PlacementConstraint
	// RequiredShares is the minimum number of pieces needed to reconstruct the
	// segment (the "min" of the redundancy scheme).
	RequiredShares int16
	// ParticipatingPieces is the number of pieces of the segment which are stored
	// on a participating node. Pieces on nodes which are missing from the overlay
	// (disqualified, exited, unknown) are not counted.
	ParticipatingPieces int16
}

// PieceCountHistogramStats holds the counters of a single histogram bucket.
type PieceCountHistogramStats struct {
	// SegmentCount is the number of segments falling into the bucket.
	SegmentCount int64
	// PieceCount is the total number of pieces of those segments, including the
	// pieces which are stored on non-participating nodes.
	PieceCount int64
	// ParticipatingPieceCount is the total number of pieces on participating nodes,
	// that is SegmentCount * PieceCountBucket.ParticipatingPieces.
	ParticipatingPieceCount int64
	// PieceBytes is the storage occupied by all the pieces counted in PieceCount.
	// Piece size is derived per segment from the redundancy scheme, so this is not
	// PieceCount times a single constant.
	PieceBytes int64
	// ParticipatingPieceBytes is the storage occupied by the pieces counted in
	// ParticipatingPieceCount, i.e. the bytes still reachable on participating nodes.
	ParticipatingPieceBytes int64
}

// PieceCountHistogramConfig holds the configuration for the PieceCountHistogram observer.
type PieceCountHistogramConfig struct {
	LogBuckets bool `help:"log every histogram bucket at the end of the loop (in addition to emitting metrics); disabled by default because the number of buckets is (placement * rs_min * TotalShares) and each bucket becomes a log line" default:"false"`
}

// PieceCountHistogram implements rangedloop.Observer.
// It builds a histogram of the number of pieces stored on participating nodes,
// grouped by (placement, redundancy required shares). Each
// (placement, required shares, participating piece count) triplet gets its own
// bucket, so the full distribution is available, not just an average.
// Every bucket reports both piece counts and the storage those pieces occupy.
type PieceCountHistogram struct {
	log     *zap.Logger
	config  PieceCountHistogramConfig
	overlay *overlay.Service

	// state that gets reset on each Start
	mu        sync.Mutex
	startTime time.Time
	histogram map[PieceCountBucket]*PieceCountHistogramStats
	// participatingNodes is pre-loaded at Start with the IDs of all participating nodes
	participatingNodes map[storj.NodeID]struct{}
	// seenBuckets remembers every bucket key that has been reported at least once,
	// across all Start/Finish cycles. In Finish we observe 0 for any bucket in
	// seenBuckets that is not present in the current histogram, so a bucket that
	// vanishes between runs no longer keeps exporting its last observed value.
	seenBuckets map[PieceCountBucket]struct{}
}

// NewPieceCountHistogram creates a new PieceCountHistogram observer.
func NewPieceCountHistogram(log *zap.Logger, overlay *overlay.Service, config PieceCountHistogramConfig) *PieceCountHistogram {
	return &PieceCountHistogram{
		log:         log,
		config:      config,
		overlay:     overlay,
		seenBuckets: make(map[PieceCountBucket]struct{}),
	}
}

// Start is called at the beginning of each segment loop.
func (o *PieceCountHistogram) Start(ctx context.Context, startTime time.Time) (err error) {
	defer monPieceCountHistogram.Task()(&ctx)(&err)

	o.mu.Lock()
	defer o.mu.Unlock()

	o.startTime = startTime
	o.histogram = make(map[PieceCountBucket]*PieceCountHistogramStats)

	// Pre-load all participating nodes to avoid querying the database for each batch.
	// Nodes that join after this point will be ignored, which is acceptable for statistics.
	nodes, err := o.overlay.GetAllParticipatingNodes(ctx)
	if err != nil {
		return err
	}

	o.participatingNodes = make(map[storj.NodeID]struct{}, len(nodes))
	for _, node := range nodes {
		o.participatingNodes[node.ID] = struct{}{}
	}

	o.log.Info("PieceCountHistogram loaded node cache",
		zap.Int("node_count", len(o.participatingNodes)))

	return nil
}

// Fork creates a new partial for processing a range.
func (o *PieceCountHistogram) Fork(ctx context.Context) (rangedloop.Partial, error) {
	return &pieceCountHistogramFork{
		observer:  o,
		histogram: make(map[PieceCountBucket]*PieceCountHistogramStats),
	}, nil
}

// Join merges partial results.
func (o *PieceCountHistogram) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer monPieceCountHistogram.Task()(&ctx)(&err)

	fork, ok := partial.(*pieceCountHistogramFork)
	if !ok {
		return nil
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	for bucket, stats := range fork.histogram {
		existing, ok := o.histogram[bucket]
		if !ok {
			o.histogram[bucket] = &PieceCountHistogramStats{
				SegmentCount:            stats.SegmentCount,
				PieceCount:              stats.PieceCount,
				ParticipatingPieceCount: stats.ParticipatingPieceCount,
				PieceBytes:              stats.PieceBytes,
				ParticipatingPieceBytes: stats.ParticipatingPieceBytes,
			}
		} else {
			existing.SegmentCount += stats.SegmentCount
			existing.PieceCount += stats.PieceCount
			existing.ParticipatingPieceCount += stats.ParticipatingPieceCount
			existing.PieceBytes += stats.PieceBytes
			existing.ParticipatingPieceBytes += stats.ParticipatingPieceBytes
		}
	}

	return nil
}

// Finish is called after all segments are processed.
func (o *PieceCountHistogram) Finish(ctx context.Context) (err error) {
	defer monPieceCountHistogram.Task()(&ctx)(&err)

	o.log.Info("PieceCountHistogram calculation complete",
		zap.Duration("duration", time.Since(o.startTime)),
		zap.Int("bucket_count", len(o.histogram)))

	for _, bucket := range o.sortedBuckets() {
		stats := o.histogram[bucket]

		o.emitBucket(bucket, stats)
		o.seenBuckets[bucket] = struct{}{}

		if o.config.LogBuckets {
			o.log.Info("piece count histogram bucket",
				zap.Uint16("placement", uint16(bucket.Placement)),
				zap.Int16("rs_min", bucket.RequiredShares),
				zap.Int16("pieces", bucket.ParticipatingPieces),
				zap.Int64("segment_count", stats.SegmentCount),
				zap.Int64("piece_count", stats.PieceCount),
				zap.Int64("participating_piece_count", stats.ParticipatingPieceCount),
				zap.Int64("piece_bytes", stats.PieceBytes),
				zap.Int64("participating_piece_bytes", stats.ParticipatingPieceBytes))
		}
	}

	// Buckets seen in previous runs but not in this one keep exporting their
	// last observed value until we overwrite them. Observing zero ensures a
	// (placement, rs_min, pieces) tag combination that no longer applies is
	// not double-counted by dashboards that sum across the pieces tag.
	var zero PieceCountHistogramStats
	for bucket := range o.seenBuckets {
		if _, ok := o.histogram[bucket]; ok {
			continue
		}
		o.emitBucket(bucket, &zero)
	}

	return nil
}

// emitBucket observes one bucket's stats as monkit metrics.
func (o *PieceCountHistogram) emitBucket(bucket PieceCountBucket, stats *PieceCountHistogramStats) {
	placementTag := monkit.NewSeriesTag("placement", placementString(bucket.Placement))
	rsMinTag := monkit.NewSeriesTag("rs_min", strconv.Itoa(int(bucket.RequiredShares)))
	piecesTag := monkit.NewSeriesTag("pieces", strconv.Itoa(int(bucket.ParticipatingPieces)))

	monPieceCountHistogram.IntVal("piece_count_histogram_segment_count",
		placementTag, rsMinTag, piecesTag).Observe(stats.SegmentCount)
	monPieceCountHistogram.IntVal("piece_count_histogram_piece_count",
		placementTag, rsMinTag, piecesTag).Observe(stats.PieceCount)
	monPieceCountHistogram.IntVal("piece_count_histogram_participating_piece_count",
		placementTag, rsMinTag, piecesTag).Observe(stats.ParticipatingPieceCount)
	monPieceCountHistogram.IntVal("piece_count_histogram_piece_bytes",
		placementTag, rsMinTag, piecesTag).Observe(stats.PieceBytes)
	monPieceCountHistogram.IntVal("piece_count_histogram_participating_piece_bytes",
		placementTag, rsMinTag, piecesTag).Observe(stats.ParticipatingPieceBytes)
}

// sortedBuckets returns the histogram keys ordered by placement, then required
// shares, then participating piece count, so that logs and metrics are emitted
// in a stable, readable order.
func (o *PieceCountHistogram) sortedBuckets() []PieceCountBucket {
	buckets := make([]PieceCountBucket, 0, len(o.histogram))
	for bucket := range o.histogram {
		buckets = append(buckets, bucket)
	}
	sort.Slice(buckets, func(i, j int) bool {
		a, b := buckets[i], buckets[j]
		if a.Placement != b.Placement {
			return a.Placement < b.Placement
		}
		if a.RequiredShares != b.RequiredShares {
			return a.RequiredShares < b.RequiredShares
		}
		return a.ParticipatingPieces < b.ParticipatingPieces
	})
	return buckets
}

// pieceCountHistogramFork implements rangedloop.Partial.
type pieceCountHistogramFork struct {
	observer  *PieceCountHistogram
	histogram map[PieceCountBucket]*PieceCountHistogramStats
}

// Process handles a batch of segments.
func (f *pieceCountHistogramFork) Process(ctx context.Context, segments []rangedloop.Segment) error {
	now := time.Now()
	for _, segment := range segments {
		if segment.Inline() {
			continue
		}

		// Skip expired segments
		if segment.Expired(now) {
			continue
		}

		if segment.Redundancy.RequiredShares <= 0 {
			continue
		}

		participating := int16(0)
		for _, piece := range segment.Pieces {
			if _, ok := f.observer.participatingNodes[piece.StorageNode]; ok {
				participating++
			}
		}

		bucket := PieceCountBucket{
			Placement:           segment.Placement,
			RequiredShares:      segment.Redundancy.RequiredShares,
			ParticipatingPieces: participating,
		}

		stats, ok := f.histogram[bucket]
		if !ok {
			stats = &PieceCountHistogramStats{}
			f.histogram[bucket] = stats
		}

		pieceSize := segment.PieceSize()

		stats.SegmentCount++
		stats.PieceCount += int64(len(segment.Pieces))
		stats.ParticipatingPieceCount += int64(participating)
		stats.PieceBytes += int64(len(segment.Pieces)) * pieceSize
		stats.ParticipatingPieceBytes += int64(participating) * pieceSize
	}

	return nil
}

var _ rangedloop.Observer = (*PieceCountHistogram)(nil)

var _ rangedloop.Partial = (*pieceCountHistogramFork)(nil)
