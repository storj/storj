// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/linkedin/goavro/v2"
	"github.com/zeebo/errs"
	"google.golang.org/api/iterator"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

var _ (RangeSplitter) = (*AvroSegmentsSplitter)(nil)
var _ (SegmentProvider) = (*AvroSegmentProvider)(nil)

var stopErr = errs.New("stop")

// AvroSegmentsSplitter segments splitter for Avro files.
type AvroSegmentsSplitter struct {
	segmentsAvroIterator    AvroReaderIterator
	nodeAliasesAvroIterator AvroReaderIterator
}

// NewAvroSegmentsSplitter creates a new AvroSegmentsSplitter.
func NewAvroSegmentsSplitter(segmentsAvroIterator, nodeAliasesAvroIterator AvroReaderIterator) RangeSplitter {
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
					nodeID, err := bytesToType(recMap["node_id"], storj.NodeIDFromBytes)
					if err != nil {
						return errs.Wrap(err)
					}

					nodeAlias, err := toInt64(recMap["node_alias"])
					if err != nil {
						return errs.Wrap(err)
					}

					nodeAliases = append(nodeAliases, metabase.NodeAliasEntry{
						ID:    nodeID,
						Alias: metabase.NodeAlias(nodeAlias),
					})
				}
			}
			return nil
		}()
		if errors.Is(err, stopErr) {
			break
		}
		if err != nil {
			return nil, errs.Wrap(err)
		}
	}

	return nodeAliases, nil
}

// AvroSegmentProvider provides segments from Avro files.
type AvroSegmentProvider struct {
	segmentsAvroIterator AvroReaderIterator
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
					segment, err := segment(ctx, recMap, p.nodeAliasCache)
					if err != nil {
						return errs.Wrap(err)
					}

					segments = append(segments, segment)

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

func segment(ctx context.Context, recMap map[string]any, aliasCache *metabase.NodeAliasCache) (Segment, error) {
	streamID, err := bytesToType(recMap["stream_id"], uuid.FromBytes)
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}

	positionEncoded, err := toInt64(recMap["position"])
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}
	position := metabase.SegmentPositionFromEncoded(uint64(positionEncoded))

	createdAt, err := toTime(recMap["created_at"])
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}

	expiresAt, err := toTimeP(recMap["expires_at"])
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}

	repairedAt, err := toTimeP(recMap["repaired_at"])
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}

	rootPieceID, err := bytesToType(recMap["root_piece_id"], storj.PieceIDFromBytes)
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}

	encryptedSize, err := toInt64(recMap["encrypted_size"])
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}
	plainOffset, err := toInt64(recMap["plain_offset"])
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}

	plainSize, err := toInt64(recMap["plain_size"])
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}

	aliasPiecesBytes, err := toBytes(recMap["remote_alias_pieces"])
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}
	var aliasPieces metabase.AliasPieces
	err = aliasPieces.SetBytes(aliasPiecesBytes)
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}

	redundancyInt64, err := toInt64(recMap["redundancy"])
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}

	var redundancy storj.RedundancyScheme
	err = redundancy.Scan(redundancyInt64)
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}

	placement, err := toInt64(recMap["placement"])
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}

	// TODO may think about memory optimization here
	pieces, err := aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
	if err != nil {
		return Segment{}, errs.Wrap(err)
	}

	return Segment{
		StreamID:      streamID,
		Position:      position,
		CreatedAt:     createdAt,
		ExpiresAt:     expiresAt,
		RepairedAt:    repairedAt,
		RootPieceID:   rootPieceID,
		EncryptedSize: int32(encryptedSize), // TODO type check
		PlainOffset:   plainOffset,
		PlainSize:     int32(plainSize), // TODO type check
		AliasPieces:   aliasPieces,
		Pieces:        pieces,
		Redundancy:    redundancy,
		Placement:     storj.PlacementConstraint(placement),
		Source:        "avro",
	}, nil
}

func toInt64(value any) (int64, error) {
	if value == nil {
		return 0, nil
	}

	switch value := value.(type) {
	case int64:
		return value, nil
	case map[string]any:
		return toInt64(value["long"])
	default:
		return 0, errs.New("unable to cast type to int64: %T", value)
	}
}

func toTime(value any) (time.Time, error) {
	t, err := toTimeP(value)
	if err != nil {
		return time.Time{}, err
	}
	if t == nil {
		return time.Time{}, nil
	}
	return *t, nil
}

func toTimeP(value any) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	switch value := value.(type) {
	case string:
		if value == "" {
			return nil, nil
		}
		t, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, errs.New("failed to parse time: %v", err)
		}
		return &t, nil
	case map[string]any:
		return toTimeP(value["string"])
	default:
		return nil, errs.New("unable to cast type to time.Time: %T", value)
	}
}

func toBytes(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}

	switch value := value.(type) {
	case []byte:
		return value, nil
	case map[string]any:
		return toBytes(value["bytes"])
	default:
		return nil, errs.New("unable to cast type to []byte: %T", value)
	}
}

func bytesToType[T any](value any, fn func([]byte) (T, error)) (result T, err error) {
	valueBytes, err := toBytes(value)
	if err != nil {
		return result, err
	}
	return fn(valueBytes)
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

// AvroReaderIterator is an iterator over Avro files.
type AvroReaderIterator interface {
	Next(ctx context.Context) (io.ReadCloser, error)
}

// AvroFileIterator is an iterator over Avro files on disk.
type AvroFileIterator struct {
	pattern string

	initOnce sync.Once

	mu           sync.Mutex
	files        []string
	currentIndex int
}

// NewAvroFileIterator creates a new AvroFileIterator.
func NewAvroFileIterator(pattern string) AvroReaderIterator {
	return &AvroFileIterator{
		pattern: pattern,
	}
}

// Next returns the next Avro file.
func (a *AvroFileIterator) Next(ctx context.Context) (_ io.ReadCloser, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.initOnce.Do(func() {
		a.files, err = filepath.Glob(a.pattern)
	})
	if err != nil {
		return nil, errs.New("failed to get files list: %v", err)
	}

	if a.currentIndex >= len(a.files) {
		return nil, nil
	}

	file, err := os.Open(a.files[a.currentIndex])
	if err != nil {
		return nil, err
	}

	a.currentIndex++

	return file, nil
}

// AvroGCSIterator is an iterator over Avro files in GCS.
type AvroGCSIterator struct {
	bucket  string
	pattern string

	initOnce sync.Once

	client   *storage.Client
	mu       sync.Mutex
	iterator *storage.ObjectIterator
}

// NewAvroGCSIterator creates a new AvroGCSIterator.
func NewAvroGCSIterator(bucket, pattern string) AvroReaderIterator {
	return &AvroGCSIterator{
		bucket:  bucket,
		pattern: pattern,
	}
}

// Next returns the next Avro file.
func (a *AvroGCSIterator) Next(ctx context.Context) (rc io.ReadCloser, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	defer func() {
		if a.client != nil && !(rc != nil && err == nil) {
			err = errs.Combine(err, a.client.Close())
		}
	}()

	a.initOnce.Do(func() {
		a.client, err = storage.NewClient(ctx)
		if err != nil {
			return
		}

		a.iterator = a.client.Bucket(a.bucket).Objects(ctx, &storage.Query{
			MatchGlob: a.pattern,
		})
	})
	if err != nil {
		return nil, errs.New("failed to create GCS storage client: %v", err)
	}

	attr, err := a.iterator.Next()
	if err != nil {
		if errors.Is(err, iterator.Done) {
			return nil, nil
		}
		return nil, errs.New("failed to get next GCS object: %v", err)
	}

	reader, err := a.client.Bucket(a.bucket).Object(attr.Name).NewReader(ctx)
	if err != nil {
		return nil, errs.New("failed to create GCS object reader: %v", err)
	}
	return reader, nil
}
