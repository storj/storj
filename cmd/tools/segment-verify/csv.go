// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
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
			fmt.Sprint(seg.Position.Encode()),
			fmt.Sprint(seg.Status.Found),
			fmt.Sprint(seg.Status.NotFound),
			fmt.Sprint(seg.Status.Retry),
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
