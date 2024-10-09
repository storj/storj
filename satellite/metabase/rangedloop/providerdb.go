// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// MetabaseRangeSplitter implements RangeSplitter.
type MetabaseRangeSplitter struct {
	db *metabase.DB

	asOfSystemInterval   time.Duration
	spannerStaleInterval time.Duration
	batchSize            int
}

// MetabaseSegmentProvider implements SegmentProvider.
type MetabaseSegmentProvider struct {
	db *metabase.DB

	uuidRange            UUIDRange
	asOfSystemInterval   time.Duration
	spannerReadTimestamp time.Time
	batchSize            int
}

// NewMetabaseRangeSplitter creates the segment provider.
func NewMetabaseRangeSplitter(db *metabase.DB, asOfSystemInterval time.Duration, spannerStaleInterval time.Duration, batchSize int) *MetabaseRangeSplitter {
	return &MetabaseRangeSplitter{
		db:                   db,
		asOfSystemInterval:   asOfSystemInterval,
		spannerStaleInterval: spannerStaleInterval,
		batchSize:            batchSize,
	}
}

// CreateRanges splits the segment table into chunks.
func (provider *MetabaseRangeSplitter) CreateRanges(nRanges int, batchSize int) ([]SegmentProvider, error) {
	uuidRanges, err := CreateUUIDRanges(uint32(nRanges))
	if err != nil {
		return nil, err
	}

	spannerReadTimestamp := time.Time{}
	if provider.spannerStaleInterval > 0 {
		spannerReadTimestamp = time.Now().Add(-provider.spannerStaleInterval)
	}

	rangeProviders := []SegmentProvider{}
	for _, uuidRange := range uuidRanges {
		rangeProviders = append(rangeProviders, &MetabaseSegmentProvider{
			db:                   provider.db,
			uuidRange:            uuidRange,
			asOfSystemInterval:   provider.asOfSystemInterval,
			spannerReadTimestamp: spannerReadTimestamp,
			batchSize:            batchSize,
		})
	}

	return rangeProviders, err
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
		BatchSize:            provider.batchSize,
		AsOfSystemInterval:   provider.asOfSystemInterval,
		StartStreamID:        startStreamID,
		EndStreamID:          endStreamID,
		SpannerReadTimestamp: provider.spannerReadTimestamp,
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
