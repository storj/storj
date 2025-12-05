// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package repaircsv

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/repair/queue"
)

func TestCsv_NewCsv(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Create a temporary input CSV file
	inputFile := filepath.Join(ctx.Dir(), "input.csv")
	createTestCSV(t, inputFile, [][]string{
		{"stream-id", "position"},
		{"11111111-1111-1111-1111-111111111111", "0"},
		{"22222222-2222-2222-2222-222222222222", "1"},
	})

	// Test successful creation
	csvQueue, err := NewQueue(Config{InputFile: inputFile}, zaptest.NewLogger(t))
	require.NoError(t, err)
	require.NotNil(t, csvQueue)
	defer csvQueue.Close()

	// Close to flush headers
	csvQueue.Close()

	// Verify output files were created
	successFile := inputFile + ".success"
	failedFile := inputFile + ".failed"
	require.FileExists(t, successFile)
	require.FileExists(t, failedFile)

	// Verify headers were written
	verifyCSVHeader(t, successFile, []string{"stream-id", "position"})
	verifyCSVHeader(t, failedFile, []string{"stream-id", "position"})
}

func TestCsv_NewCsv_InvalidInputFile(t *testing.T) {
	// Test with non-existent input file
	_, err := NewQueue(Config{InputFile: "/non/existent/file.csv"}, zaptest.NewLogger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to open input file")
}

func TestCsv_Select(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Create test CSV with various cases
	inputFile := filepath.Join(ctx.Dir(), "input.csv")
	createTestCSV(t, inputFile, [][]string{
		{"stream-id", "position"},
		{"11111111-1111-1111-1111-111111111111", "0"},
		{"22222222-2222-2222-2222-222222222222", "1"},
		{"33333333-3333-3333-3333-333333333333", "2"},
		{"invalid-uuid", "3"},                                        // This should be skipped
		{"44444444-4444-4444-4444-444444444444", "invalid-position"}, // This should be skipped
		{"55555555-5555-5555-5555-555555555555", "5"},
	})

	csvQueue, err := NewQueue(Config{InputFile: inputFile}, zaptest.NewLogger(t))
	require.NoError(t, err)
	defer csvQueue.Close()

	// Test selecting with limit
	segments, err := csvQueue.Select(t.Context(), 3, nil, nil)
	require.NoError(t, err)
	require.Len(t, segments, 3)

	// Verify the segments
	expectedUUIDs := []string{
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
		"33333333-3333-3333-3333-333333333333",
	}
	expectedPositions := []uint64{0, 1, 2}

	for i, segment := range segments {
		require.Equal(t, expectedUUIDs[i], segment.StreamID.String())
		require.Equal(t, expectedPositions[i], segment.Position.Encode())
		require.Equal(t, float64(0.0), segment.SegmentHealth)
		require.Equal(t, storj.PlacementConstraint(0), segment.Placement)
		require.Nil(t, segment.AttemptedAt)
	}

	// Test selecting remaining segments
	segments, err = csvQueue.Select(t.Context(), 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, segments, 1) // Only one valid segment left (invalid ones are skipped)
	require.Equal(t, "55555555-5555-5555-5555-555555555555", segments[0].StreamID.String())
	require.Equal(t, uint64(5), segments[0].Position.Encode())

	// Test EOF - no more segments
	segments, err = csvQueue.Select(t.Context(), 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, segments, 0)
}

func TestCsv_Select_NoHeader(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Create test CSV without header
	inputFile := filepath.Join(ctx.Dir(), "input.csv")
	createTestCSV(t, inputFile, [][]string{
		{"11111111-1111-1111-1111-111111111111", "0"},
		{"22222222-2222-2222-2222-222222222222", "1"},
	})

	csvQueue, err := NewQueue(Config{InputFile: inputFile}, zaptest.NewLogger(t))
	require.NoError(t, err)
	defer csvQueue.Close()

	segments, err := csvQueue.Select(t.Context(), 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, segments, 2)

	require.Equal(t, "11111111-1111-1111-1111-111111111111", segments[0].StreamID.String())
	require.Equal(t, uint64(0), segments[0].Position.Encode())
}

func TestCsv_Select_EmptyFile(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Create empty CSV file
	inputFile := filepath.Join(ctx.Dir(), "input.csv")
	createTestCSV(t, inputFile, [][]string{})

	csvQueue, err := NewQueue(Config{InputFile: inputFile}, zaptest.NewLogger(t))
	require.NoError(t, err)
	defer csvQueue.Close()

	segments, err := csvQueue.Select(t.Context(), 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, segments, 0)
}

func TestCsv_Release(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Create test CSV
	inputFile := filepath.Join(ctx.Dir(), "input.csv")
	createTestCSV(t, inputFile, [][]string{
		{"stream-id", "position"},
		{"11111111-1111-1111-1111-111111111111", "0"},
	})

	csvQueue, err := NewQueue(Config{InputFile: inputFile}, zaptest.NewLogger(t))
	require.NoError(t, err)
	defer csvQueue.Close()

	// Create test segments
	streamID1 := uuid.UUID{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11}
	streamID2 := uuid.UUID{0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22}

	segment1 := queue.InjuredSegment{
		StreamID: streamID1,
		Position: metabase.SegmentPositionFromEncoded(0),
	}
	segment2 := queue.InjuredSegment{
		StreamID: streamID2,
		Position: metabase.SegmentPositionFromEncoded(1),
	}

	// Test successful repair
	err = csvQueue.Release(t.Context(), segment1, true)
	require.NoError(t, err)

	// Test failed repair
	err = csvQueue.Release(t.Context(), segment2, false)
	require.NoError(t, err)

	// Close to flush writes
	csvQueue.Close()

	// Verify success file content
	successFile := inputFile + ".success"
	successContent := readCSV(t, successFile)
	require.Len(t, successContent, 2) // Header + 1 record
	require.Equal(t, []string{"stream-id", "position"}, successContent[0])
	require.Equal(t, []string{streamID1.String(), "0"}, successContent[1])

	// Verify failed file content
	failedFile := inputFile + ".failed"
	failedContent := readCSV(t, failedFile)
	require.Len(t, failedContent, 2) // Header + 1 record
	require.Equal(t, []string{"stream-id", "position"}, failedContent[0])
	require.Equal(t, []string{streamID2.String(), "1"}, failedContent[1])
}

func TestCsv_InterfaceCompliance(t *testing.T) {
	// Verify that Queue implements queue.Consumer interface
	var _ queue.Consumer = (*Queue)(nil)
}

func TestCsv_Close(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	inputFile := filepath.Join(ctx.Dir(), "input.csv")
	createTestCSV(t, inputFile, [][]string{
		{"stream-id", "position"},
		{"11111111-1111-1111-1111-111111111111", "0"},
	})

	csvQueue, err := NewQueue(Config{InputFile: inputFile}, zaptest.NewLogger(t))
	require.NoError(t, err)

	// Close should not panic and should be callable multiple times
	csvQueue.Close()
	csvQueue.Close() // Second close should not panic
}

// Helper functions

func createTestCSV(t *testing.T, filename string, data [][]string) {
	file, err := os.Create(filename)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, record := range data {
		err := writer.Write(record)
		require.NoError(t, err)
	}
}

func readCSV(t *testing.T, filename string) [][]string {
	file, err := os.Open(filename)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	return records
}

func verifyCSVHeader(t *testing.T, filename string, expectedHeader []string) {
	records := readCSV(t, filename)
	require.Greater(t, len(records), 0, "CSV file should have at least a header")
	require.Equal(t, expectedHeader, records[0], "Header should match expected")
}
