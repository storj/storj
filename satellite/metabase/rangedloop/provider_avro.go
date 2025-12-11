// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"errors"

	"github.com/linkedin/goavro/v2"
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/avrometabase"
)

var _ (RangeSplitter) = (*AvroSegmentsSplitter)(nil)
var _ (SegmentProvider) = (*AvroSegmentProvider)(nil)

var stopErr = errs.New("stop")

// AvroSegmentsSplitter segments splitter for Avro files.
type AvroSegmentsSplitter struct {
	segmentsAvroIterator    avrometabase.ReaderIterator
	nodeAliasesAvroIterator avrometabase.ReaderIterator
}

// NewAvroSegmentsSplitter creates a new AvroSegmentsSplitter.
func NewAvroSegmentsSplitter(segmentsAvroIterator, nodeAliasesAvroIterator avrometabase.ReaderIterator) RangeSplitter {
	return &AvroSegmentsSplitter{
		segmentsAvroIterator:    segmentsAvroIterator,
		nodeAliasesAvroIterator: nodeAliasesAvroIterator,
	}
}

// CreateRanges creates ranges for the given number of ranges and batch size.
func (s *AvroSegmentsSplitter) CreateRanges(ctx context.Context, nRanges int, batchSize int) (_ []SegmentProvider, err error) {
	defer mon.Task()(&ctx)(&err)

	if nRanges <= 0 {
		nRanges = 1
	}

	nodeAliases, err := s.readNodeAliases(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	providers := make([]SegmentProvider, nRanges)
	for i := 0; i < nRanges; i++ {
		providers[i] = &AvroSegmentProvider{
			segmentsAvroIterator: s.segmentsAvroIterator,
			nodeAliasCache:       metabase.NewNodeAliasCache(&readOnlyNodeAliasDB{entries: nodeAliases}, true),
			batchSize:            batchSize,
		}
	}

	return providers, nil
}

func (s *AvroSegmentsSplitter) readNodeAliases(ctx context.Context) ([]metabase.NodeAliasEntry, error) {
	nodeAliases := []metabase.NodeAliasEntry{}

	for {
		err := func() error {
			reader, err := s.nodeAliasesAvroIterator.Next(ctx)
			if err != nil {
				return err
			}

			if reader == nil {
				return stopErr
			}

			defer func() {
				err = errs.Combine(err, reader.Close())
			}()

			ocfReader, err := goavro.NewOCFReader(reader)
			if err != nil {
				return errs.New("failed to create Avro reader: %v", err)
			}

			for ocfReader.Scan() {
				record, err := ocfReader.Read()
				if err != nil {
					return errs.New("failed to read Avro record: %v", err)
				}

				if recMap, ok := record.(map[string]any); ok {
					nodeAlias, err := avrometabase.NodeAliasFromRecord(ctx, recMap)
					if err != nil {
						return errs.Wrap(err)
					}

					nodeAliases = append(nodeAliases, nodeAlias)
				}
			}
			return nil
		}()
		if errors.Is(err, stopErr) {
			break
		}
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	return nodeAliases, nil
}

// AvroSegmentProvider provides segments from Avro files.
type AvroSegmentProvider struct {
	segmentsAvroIterator avrometabase.ReaderIterator
	nodeAliasCache       *metabase.NodeAliasCache

	batchSize int
}

// Range returns a provider range.
// It is always empty because with Avro files we cannot do exact split.
func (p *AvroSegmentProvider) Range() UUIDRange {
	return UUIDRange{}
}

// Iterate iterates over segments.
func (p *AvroSegmentProvider) Iterate(ctx context.Context, fn func([]Segment) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err := func() error {
			reader, err := p.segmentsAvroIterator.Next(ctx)
			if err != nil {
				return err
			}
			if reader == nil {
				return stopErr
			}

			defer func() {
				err = errs.Combine(err, reader.Close())
			}()

			ocfr, err := goavro.NewOCFReader(reader)
			if err != nil {
				return errs.New("failed to create Avro reader: %v", err)
			}

			segments := make([]Segment, 0, p.batchSize)
			for ocfr.Scan() {
				record, err := ocfr.Read()
				if err != nil {
					return errs.New("failed to read Avro record: %v", err)
				}

				if recMap, ok := record.(map[string]any); ok {
					entry, err := avrometabase.SegmentFromRecord(ctx, recMap, p.nodeAliasCache)
					if err != nil {
						return errs.Wrap(err)
					}

					segments = append(segments, Segment(entry))

					if len(segments) >= p.batchSize {
						if err := fn(segments); err != nil {
							return err
						}
						segments = segments[:0]
					}
				} else {
					return errs.New("Avro record is not a map[string]any")
				}
			}

			if len(segments) > 0 {
				if err := fn(segments); err != nil {
					return err
				}
			}

			return nil
		}()
		if errors.Is(err, stopErr) {
			break
		}
		if err != nil {
			return errs.Wrap(err)
		}
	}

	return nil
}

type readOnlyNodeAliasDB struct {
	entries []metabase.NodeAliasEntry
}

func (db *readOnlyNodeAliasDB) EnsureNodeAliases(ctx context.Context, opts metabase.EnsureNodeAliases) error {
	return errs.New("not implemented")
}

func (db *readOnlyNodeAliasDB) ListNodeAliases(ctx context.Context) (_ []metabase.NodeAliasEntry, err error) {
	return db.entries, nil
}
func (db *readOnlyNodeAliasDB) GetNodeAliasEntries(ctx context.Context, opts metabase.GetNodeAliasEntries) (_ []metabase.NodeAliasEntry, err error) {
	return nil, errs.New("not implemented")
}
