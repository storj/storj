// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// MetabaseRangeSplitter implements RangeSplitter.
type MetabaseRangeSplitter struct {
	log *zap.Logger
	db  *metabase.DB

	config                Config
	overrideReadTimestamp time.Time
	warnedLiveReads       sync.Once
}

// MetabaseSegmentProvider implements SegmentProvider.
type MetabaseSegmentProvider struct {
	db *metabase.DB

	uuidRange          UUIDRange
	asOfSystemInterval time.Duration
	readTimestamp      time.Time
	allowLiveReads     bool
	batchSize          int
}

// NewMetabaseRangeSplitter creates the segment provider.
func NewMetabaseRangeSplitter(log *zap.Logger, db *metabase.DB, config Config) *MetabaseRangeSplitter {
	return NewMetabaseRangeSplitterWithReadTimestamp(log, db, config, time.Time{})
}

// NewMetabaseRangeSplitterWithReadTimestamp creates the segment provider reading
// a consistent snapshot of the database at the given timestamp.
func NewMetabaseRangeSplitterWithReadTimestamp(log *zap.Logger, db *metabase.DB, config Config, overrideReadTimestamp time.Time) *MetabaseRangeSplitter {
	return &MetabaseRangeSplitter{
		log:                   log,
		db:                    db,
		config:                config,
		overrideReadTimestamp: overrideReadTimestamp,
	}
}

// CreateRanges splits the segment table into chunks.
func (provider *MetabaseRangeSplitter) CreateRanges(ctx context.Context, nRanges int, batchSize int) ([]SegmentProvider, error) {
	uuidRanges, err := CreateUUIDRanges(uint32(nRanges))
	if err != nil {
		return nil, err
	}

	// batchSize comes from operator configuration, while the iterator rejects
	// anything above its maximum. Cap it here so that a misconfigured satellite
	// keeps making progress instead of failing every run, and so that the size we
	// group the entries by stays the size the iterator really pages at: the
	// iterator recycles its buffers once a page has been handed over, and a group
	// spanning two pages would carry entries it has already overwritten.
	if batchSize > metabase.MaxLoopIteratorBatchSize {
		provider.log.Warn("configured batch size is above the maximum, using the maximum instead",
			zap.Int("configured", batchSize),
			zap.Int("batch_size", metabase.MaxLoopIteratorBatchSize),
		)
		batchSize = metabase.MaxLoopIteratorBatchSize
	}

	readTimestamp := provider.overrideReadTimestamp
	if readTimestamp.IsZero() && provider.config.StaleInterval > 0 {
		readTimestamp = time.Now().Add(-provider.config.StaleInterval)
	}

	if !readTimestamp.IsZero() {
		provider.log.Info("Setting fixed read timestamp", zap.Time("timestamp", readTimestamp))
	}
	if provider.config.AllowLiveReads {
		// whether the scan reads live is a property of the metabase, not of
		// the pass, so say it once rather than on every pass of a continuous loop
		provider.warnedLiveReads.Do(func() {
			WarnLiveReads(provider.log, provider.db.Implementations())
		})
	}

	rangeProviders := []SegmentProvider{}
	for _, uuidRange := range uuidRanges {
		rangeProviders = append(rangeProviders, &MetabaseSegmentProvider{
			db:                 provider.db,
			uuidRange:          uuidRange,
			asOfSystemInterval: provider.config.AsOfSystemInterval,
			readTimestamp:      readTimestamp,
			allowLiveReads:     provider.config.AllowLiveReads,
			batchSize:          batchSize,
		})
	}

	return rangeProviders, err
}

// ReadsSnapshot reports whether the scan reads one snapshot of the whole
// metabase: at one fixed timestamp, one the caller pinned or one derived from
// the stale interval, that every backend serves; or live, where live reads
// were allowed and no backend could have served a timestamp anyway, which is
// a single Postgres backend for testing or a restored backup, one snapshot by
// construction. See CanReadSnapshot.
func (provider *MetabaseRangeSplitter) ReadsSnapshot() bool {
	fixed := !provider.overrideReadTimestamp.IsZero() || provider.config.StaleInterval > 0
	return CanReadSnapshot(fixed, provider.config.AllowLiveReads, provider.db.Implementations())
}

// Range returns range which is processed by this provider.
func (provider *MetabaseSegmentProvider) Range() UUIDRange {
	return provider.uuidRange
}

// Iterate loops over a part of the segment table.
func (provider *MetabaseSegmentProvider) Iterate(ctx context.Context, fn func([]Segment) error) error {
	var startStreamID uuid.UUID
	var endStreamID uuid.UUID

	if provider.uuidRange.Start != nil {
		startStreamID = *provider.uuidRange.Start
	}
	if provider.uuidRange.End != nil {
		endStreamID = *provider.uuidRange.End
	}

	return provider.db.IterateLoopSegments(ctx, metabase.IterateLoopSegments{
		BatchSize:          provider.batchSize,
		AsOfSystemInterval: provider.asOfSystemInterval,
		StartStreamID:      startStreamID,
		EndStreamID:        endStreamID,
		ReadTimestamp:      provider.readTimestamp,
		AllowLiveReads:     provider.allowLiveReads,
	}, func(ctx context.Context, iterator metabase.LoopSegmentsIterator) error {
		segments := make([]Segment, 0, provider.batchSize)

		segment := metabase.LoopSegmentEntry{}
		for iterator.Next(ctx, &segment) {
			err := ctx.Err()
			if err != nil {
				return err
			}

			segments = append(segments, Segment(segment))

			if len(segments) >= provider.batchSize {
				err = fn(segments)
				if err != nil {
					return err
				}
				// prepare for next batch
				segments = segments[:0]
			}
		}

		// send last batch
		if len(segments) > 0 {
			return fn(segments)
		}

		return nil
	})
}
