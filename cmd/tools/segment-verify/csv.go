// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
)

// CSVWriter writes segments to a file.
type CSVWriter struct {
	header bool
	file   io.WriteCloser
	wr     *csv.Writer
}

var _ SegmentWriter = (*CSVWriter)(nil)

// NewCSVWriter creates a new segment writer that writes to the specified path.
func NewCSVWriter(path string) (*CSVWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &CSVWriter{
		file: f,
		wr:   csv.NewWriter(f),
	}, nil
}

// NewCustomCSVWriter creates a new segment writer that writes to the io.Writer.
func NewCustomCSVWriter(w io.Writer) *CSVWriter {
	return &CSVWriter{
		file: nopCloser{w},
		wr:   csv.NewWriter(w),
	}
}

// Close closes the writer.
func (csv *CSVWriter) Close() error {
	return Error.Wrap(csv.file.Close())
}

// Write writes and flushes the segments.
func (csv *CSVWriter) Write(ctx context.Context, segments []*Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !csv.header {
		csv.header = true
		err := csv.wr.Write([]string{
			"stream id",
			"position",
			"created_at",
			"required",
			"found",
			"not found",
			"retry",
		})
		if err != nil {
			return Error.Wrap(err)
		}
	}

	defer csv.wr.Flush()

	for _, seg := range segments {
		if ctx.Err() != nil {
			return Error.Wrap(err)
		}

		err := csv.wr.Write([]string{
			seg.StreamID.String(),
			strconv.FormatUint(seg.Position.Encode(), 10),
			seg.CreatedAt.Format(time.RFC3339),
			strconv.Itoa(int(seg.Redundancy.RequiredShares)),
			strconv.Itoa(int(seg.Status.Found)),
			strconv.Itoa(int(seg.Status.NotFound)),
			strconv.Itoa(int(seg.Status.Retry)),
		})
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// nopCloser adds Close method to a writer.
type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

// pieceCSVWriter writes pieces and their outcomes from fetching to a file.
type pieceCSVWriter struct {
	header bool
	file   io.WriteCloser
	wr     *csv.Writer
	mu     sync.Mutex
}

// newPieceCSVWriter creates a new piece CSV writer that writes to the specified path.
func newPieceCSVWriter(path string) (*pieceCSVWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &pieceCSVWriter{
		file: f,
		wr:   csv.NewWriter(f),
	}, nil
}

// Close closes the writer.
func (csv *pieceCSVWriter) Close() error {
	return Error.Wrap(csv.file.Close())
}

// Write writes and flushes the segments.
func (csv *pieceCSVWriter) Write(
	ctx context.Context,
	segment *metabase.VerifySegment,
	nodeID storj.NodeID,
	pieceNum int,
	outcome audit.Outcome,
) (err error) {
	defer mon.Task()(&ctx)(&err)

	csv.mu.Lock()
	defer csv.mu.Unlock()

	defer csv.wr.Flush()

	if !csv.header {
		csv.header = true
		err := csv.wr.Write([]string{
			"stream id",
			"position",
			"created_at",
			"node id",
			"piece number",
			"outcome",
		})
		if err != nil {
			return Error.Wrap(err)
		}
	}

	if ctx.Err() != nil {
		return Error.Wrap(ctx.Err())
	}

	err = csv.wr.Write([]string{
		segment.StreamID.String(),
		strconv.FormatUint(segment.Position.Encode(), 10),
		segment.CreatedAt.Format(time.RFC3339),
		nodeID.String(),
		strconv.Itoa(pieceNum),
		outcomeString(outcome),
	})
	return Error.Wrap(err)
}

func outcomeString(outcome audit.Outcome) string {
	switch outcome {
	case audit.OutcomeSuccess:
		return "SUCCESS"
	case audit.OutcomeUnknownError:
		return "UNKNOWN_ERROR"
	case audit.OutcomeNodeOffline:
		return "NODE_OFFLINE"
	case audit.OutcomeFailure:
		return "NOT_FOUND"
	case audit.OutcomeNotPerformed:
		return "RETRY"
	case audit.OutcomeTimedOut:
		return "TIMED_OUT"
	}
	return fmt.Sprintf("(unexpected outcome code %d)", outcome)
}

// SegmentCSVSource reads from a CSV file that has segment_id,position as the first two columns
// (such as, for example, the segments-retry.csv and segments-not-found.csv output files).
type SegmentCSVSource struct {
	csvFile   io.ReadCloser
	csvReader *csv.Reader
}

// OpenSegmentCSVFile opens a CSV file for reading. The CSV file should have segment_id,position
// as the first two columns.
func OpenSegmentCSVFile(path string) (_ *SegmentCSVSource, err error) {
	csvFile, err := os.Open(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	csvReader := csv.NewReader(csvFile)
	return &SegmentCSVSource{
		csvFile:   csvFile,
		csvReader: csvReader,
	}, nil
}

// Close closes a SegmentCSVSource.
func (s *SegmentCSVSource) Close() error {
	return s.csvFile.Close()
}

// Next returns the next segment from the CSV file. If there are no more, it
// returns (nil, io.EOF).
func (s *SegmentCSVSource) Next() (*metabase.GetSegmentByPosition, error) {
	entry, err := s.csvReader.Read()
	if err != nil {
		return nil, err
	}
	segmentUUID, err := uuid.FromString(entry[0])
	if err != nil {
		return nil, Error.New("segment-id encoding: %w", err)
	}
	positionEncoded, err := strconv.ParseUint(entry[1], 10, 64)
	if err != nil {
		return nil, Error.New("position encoding: %w", err)
	}
	return &metabase.GetSegmentByPosition{
		StreamID: segmentUUID,
		Position: metabase.SegmentPositionFromEncoded(positionEncoded),
	}, nil
}
