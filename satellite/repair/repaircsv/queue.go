// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package repaircsv

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/repair/queue"
)

// Queue implements queue.Consumer interface for CSV-based segment repair tracking.
type Queue struct {
	inputFile     *os.File
	inputReader   *csv.Reader
	headerSkipped bool
	successFile   *os.File
	failedFile    *os.File
	successWriter *csv.Writer
	failedWriter  *csv.Writer
	log           *zap.Logger
}

var _ queue.Consumer = (*Queue)(nil)

// Record represents one line from the CSV
type Record struct {
	SegmentID string `csv:"segment_id"`
	Position  int    `csv:"position"`
	Placement int    `csv:"placement"`
}

// Config holds configuration for CSV queue.
type Config struct {
	InputFile string `usage:"Path to the input CSV file containing segments to repair (stream_id, position, placement)"`
}

// NewQueue creates a new CSV queue consumer that reads from inputFile and writes results to successFile and failedFile.
func NewQueue(cfg Config, log *zap.Logger) (*Queue, error) {
	// Open input CSV file
	inputF, err := os.Open(cfg.InputFile)
	if err != nil {
		return nil, errs.New("failed to open input file: %w", err)
	}
	inputReader := csv.NewReader(inputF)

	successF, err := os.Create(cfg.InputFile + ".success")
	if err != nil {
		_ = inputF.Close()
		return nil, errs.New("failed to create success file: %w", err)
	}
	successWriter := csv.NewWriter(successF)

	if err := successWriter.Write([]string{"stream-id", "position"}); err != nil {
		_ = inputF.Close()
		_ = successF.Close()
		return nil, errs.New("failed to write success file header: %w", err)
	}

	failedF, err := os.Create(cfg.InputFile + ".failed")
	if err != nil {
		_ = inputF.Close()
		_ = successF.Close()
		return nil, errs.New("failed to create failed file: %w", err)
	}

	failedWriter := csv.NewWriter(failedF)
	if err := failedWriter.Write([]string{"stream-id", "position"}); err != nil {
		_ = inputF.Close()
		_ = successF.Close()
		_ = failedF.Close()
		return nil, errs.New("failed to write failed file header: %w", err)
	}

	return &Queue{
		inputFile:     inputF,
		inputReader:   inputReader,
		headerSkipped: false,
		successFile:   successF,
		failedFile:    failedF,
		successWriter: successWriter,
		failedWriter:  failedWriter,
		log:           log,
	}, nil
}

// Select implements queue.Consumer interface.
func (c *Queue) Select(ctx context.Context, limit int, includedPlacements []storj.PlacementConstraint, excludedPlacements []storj.PlacementConstraint) ([]queue.InjuredSegment, error) {
	var result []queue.InjuredSegment

	for len(result) < limit {
		record, err := c.inputReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(result) == 0 {
					c.log.Info("No more records in the CSV file")
					return nil, nil
				} else {
					return result, nil
				}

			}
			return nil, errs.New("failed to read CSV record: %w", err)
		}

		// Skip header row if we haven't processed it yet
		if !c.headerSkipped {
			c.headerSkipped = true
			if len(record) > 0 && (record[0] == "stream-id" || record[0] == "stream_id") {
				continue
			}
		}

		// Parse the record
		if len(record) < 2 {
			c.log.Warn("Skipping line without enough records", zap.Strings("line", record))
			continue // Skip invalid records
		}

		streamID, err := uuid.FromString(record[0])
		if err != nil {
			c.log.Warn("Skipping line with invalid SegmentID", zap.String("segment_id", record[0]))
			continue // Skip invalid stream IDs
		}

		position, err := strconv.ParseUint(record[1], 10, 64)
		if err != nil {
			c.log.Warn("Skipping line with invalid position", zap.String("segment_id", record[1]))
			continue // Skip invalid positions
		}

		injuredSegment := queue.InjuredSegment{
			StreamID:      streamID,
			Position:      metabase.SegmentPositionFromEncoded(position),
			SegmentHealth: 0.0, // Default health for CSV input
			AttemptedAt:   nil,
			UpdatedAt:     time.Now(),
			InsertedAt:    time.Now(),
			Placement:     storj.PlacementConstraint(0), // Default placement
		}

		result = append(result, injuredSegment)
	}

	return result, nil
}

// Release implements queue.Consumer interface.
func (c *Queue) Release(ctx context.Context, s queue.InjuredSegment, repaired bool) error {
	if repaired {
		return c.successWriter.Write([]string{
			s.StreamID.String(),
			strconv.FormatUint(s.Position.Encode(), 10),
		})
	} else {
		return c.failedWriter.Write([]string{
			s.StreamID.String(),
			strconv.FormatUint(s.Position.Encode(), 10),
		})
	}
}

// Close closes all file handles and flushes any remaining data.
func (c *Queue) Close() {
	if c.successWriter != nil {
		c.successWriter.Flush()
	}
	if c.failedWriter != nil {
		c.failedWriter.Flush()
	}
	if c.inputFile != nil {
		_ = c.inputFile.Close()
	}
	if c.successFile != nil {
		_ = c.successFile.Close()
	}
	if c.failedFile != nil {
		_ = c.failedFile.Close()
	}
}

// SegmentWithPosition represents a segment identifier with stream ID and position.
type SegmentWithPosition struct {
	StreamID uuid.UUID
	Position uint64
}
