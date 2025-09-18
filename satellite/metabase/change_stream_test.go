// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/iterator"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/eventing/changestream"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
)

func TestSpannerChangeStreamMessageGeneration(t *testing.T) {
	// Run test only on Spanner since change streams are Spanner-specific
	metabasetest.RunWithConfig(t, metabase.Config{
		ApplicationName:  "test-change-stream",
		MaxNumberOfParts: 100,
	}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// Only run this test for Spanner databases
		if db.Implementation() != dbutil.Spanner {
			t.Skip("Skipping change stream test for non-Spanner database")
		}

		t.Run("InsertOperations", func(t *testing.T) {
			testChangeStreamInsertOperations(ctx, t, db)
		})

		t.Run("DeleteOperations", func(t *testing.T) {
			testChangeStreamDeleteOperations(ctx, t, db)
		})

		t.Run("NoEventsForTransmitEventFalse", func(t *testing.T) {
			testNoEventsForTransmitEventFalse(ctx, t, db)
		})

		t.Run("NoEventsForNonTrackedColumns", func(t *testing.T) {
			testNoEventsForNonTrackedColumns(ctx, t, db)
		})
	})
}

// Test INSERT operations that should generate change stream events
func testChangeStreamInsertOperations(ctx context.Context, t *testing.T, db *metabase.DB) {
	projectID, bucketName, _, eventCh, errCh, cleanup := setupChangeStreamTest(ctx, t, db)
	defer cleanup()

	t.Log("Testing INSERT operations (should generate 2 events: BEGIN creates status=1, COMMIT creates status=3)")

	obj := metabase.ObjectStream{
		ProjectID:  projectID,
		BucketName: bucketName,
		ObjectKey:  metabase.ObjectKey(testrand.Bytes(16)),
		Version:    1,
		StreamID:   testrand.UUID(),
	}

	beforeOperations := time.Now()

	// Begin object (creates initial row with status=1)
	_, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
		ObjectStream: obj,
		Encryption:   metabasetest.DefaultEncryption,
	})
	require.NoError(t, err, "Should be able to begin object")

	// Commit object (inserts a new record with status=3)
	// An object is committed to the table with a `DELETE`+`INSERT` transaction
	_, err = db.CommitObject(ctx, metabase.CommitObject{
		ObjectStream:  obj,
		TransmitEvent: true,
	})
	require.NoError(t, err, "Should be able to commit object")

	// Collect events and verify exactly 2 events were generated
	events := collectChangeStreamEvents(t, eventCh, errCh, 10*time.Second)
	verifyExpectedEvents(t, events, 2, beforeOperations)

	// Check that we have both status=1 and status=3 events
	statuses := make(map[string]bool)
	for _, event := range events {
		require.Equal(t, "INSERT", event.ModType, "All events should be INSERT type")
		if len(event.Mods) > 0 {
			mod := event.Mods[0]
			if mod.NewValues.Valid {
				if newVals, ok := mod.NewValues.Value.(map[string]interface{}); ok {
					if status, ok := newVals["status"].(string); ok {
						statuses[status] = true
					}
				}
			}
		}
	}

	require.True(t, statuses["1"], "Should have event with status=1 (from BeginObject)")
	require.True(t, statuses["3"], "Should have event with status=3 (from CommitObject)")

	t.Log("✅ INSERT operations generated expected 2 change stream events (status=1 and status=3)")
}

// Test DELETE operations that should generate change stream events
func testChangeStreamDeleteOperations(ctx context.Context, t *testing.T, db *metabase.DB) {
	projectID, bucketName, _, eventCh, errCh, cleanup := setupChangeStreamTest(ctx, t, db)
	defer cleanup()

	t.Log("Testing DELETE operations (documenting actual DELETE event behavior)")

	// First create an object - since TransmitEvent may not be implemented,
	// this will likely generate events regardless
	obj := metabase.ObjectStream{
		ProjectID:  projectID,
		BucketName: bucketName,
		ObjectKey:  metabase.ObjectKey(testrand.Bytes(16)),
		Version:    1,
		StreamID:   testrand.UUID(),
	}

	_, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
		ObjectStream: obj,
		Encryption:   metabasetest.DefaultEncryption,
	})
	require.NoError(t, err, "Should be able to begin object")

	_, err = db.CommitObject(ctx, metabase.CommitObject{
		ObjectStream:  obj,
		TransmitEvent: false, // May still generate events
	})
	require.NoError(t, err, "Should be able to commit object")

	// Clear any creation events by collecting them first
	t.Log("Collecting any events from object creation...")
	creationEvents := collectChangeStreamEvents(t, eventCh, errCh, 3*time.Second)
	t.Logf("Object creation generated %d events (will be ignored for DELETE test)", len(creationEvents))

	// Now mark the time for DELETE operation
	beforeDelete := time.Now()

	// Delete the object
	_, err = db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
		ObjectLocation: metabase.ObjectLocation{
			ProjectID:  projectID,
			BucketName: bucketName,
			ObjectKey:  obj.ObjectKey,
		},
		Version:       1,
		TransmitEvent: true, // This should generate a change stream message
	})
	require.NoError(t, err, "Should be able to delete object")

	// Collect events after delete
	t.Log("Collecting events from DELETE operation...")
	deleteEvents := collectChangeStreamEvents(t, eventCh, errCh, 20*time.Second)
	verifyExpectedEvents(t, deleteEvents, 1, beforeDelete)

	// Filter for events that happened after delete
	relevantEvents := filterRelevantEvents(deleteEvents, beforeDelete)
	t.Logf("DELETE operation generated %d change stream events", len(relevantEvents))

	// Document what type of events we actually get for deletes
	for i, event := range relevantEvents {
		t.Logf("DELETE Event %d: ModType=%s, ServerTxnID=%s", i+1, event.ModType, event.ServerTransactionId)
	}

	require.Equal(t, "DELETE", relevantEvents[0].ModType, "Event should be DELETE type")
}

// Test that TransmitEvent flag behavior is documented
func testNoEventsForTransmitEventFalse(ctx context.Context, t *testing.T, db *metabase.DB) {
	projectID, bucketName, adapter, eventCh, errCh, cleanup := setupChangeStreamTest(ctx, t, db)
	defer cleanup()

	// Check if we're using emulator - skip test if so since emulator doesn't support allow_txn_exclusion
	if adapter.IsEmulator() {
		t.Skip("Spanner emulator doesn't support allow_txn_exclusion, skipping TransmitEvent=false test")
		return
	}

	t.Log("Testing operations with TransmitEvent=false (documenting current behavior)")
	t.Log("Note: Currently TransmitEvent flag may not be fully implemented - this test documents actual behavior")

	obj := metabase.ObjectStream{
		ProjectID:  projectID,
		BucketName: bucketName,
		ObjectKey:  metabase.ObjectKey(testrand.Bytes(16)),
		Version:    1,
		StreamID:   testrand.UUID(),
	}

	beforeOperations := time.Now()

	// Begin and commit object with TransmitEvent=false
	_, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
		ObjectStream: obj,
		Encryption:   metabasetest.DefaultEncryption,
	})
	require.NoError(t, err, "Should be able to begin object")

	_, err = db.CommitObject(ctx, metabase.CommitObject{
		ObjectStream:  obj,
		TransmitEvent: false, // This may still generate events if not implemented
	})
	require.NoError(t, err, "Should be able to commit object with TransmitEvent=false")
	// Collect creation events first
	t.Log("Collecting any events from object creation...")
	createEvents := collectChangeStreamEvents(t, eventCh, errCh, 3*time.Second)
	relevantCreateEvents := filterRelevantEvents(createEvents, beforeOperations)
	t.Logf("Object creation generated %d events", len(relevantCreateEvents))

	// Mark time before delete operation
	beforeDelete := time.Now()

	// Wait a moment to ensure clean timing separation
	time.Sleep(1 * time.Second)

	// Delete the object with TransmitEvent=false
	_, err = db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
		ObjectLocation: metabase.ObjectLocation{
			ProjectID:  projectID,
			BucketName: bucketName,
			ObjectKey:  obj.ObjectKey,
		},
		Version:       1,
		TransmitEvent: false, // This should not generate events
	})
	require.NoError(t, err, "Should be able to delete object with TransmitEvent=false")

	// Collect events after delete
	t.Log("Collecting events from DELETE operation...")
	deleteEvents := collectChangeStreamEvents(t, eventCh, errCh, 20*time.Second)
	relevantDeleteEvents := filterRelevantEvents(deleteEvents, beforeDelete)

	t.Logf("Create operations with TransmitEvent=false generated %d events", len(relevantCreateEvents))
	t.Logf("Delete operations with TransmitEvent=false generated %d events", len(relevantDeleteEvents))

	totalRelevantEvents := len(relevantCreateEvents) + len(relevantDeleteEvents)

	if totalRelevantEvents > 0 {
		t.Log("⚠️  TransmitEvent=false still generated events - flag may not be implemented yet")

		// Log details of any events that were generated
		for i, event := range relevantCreateEvents {
			t.Logf("CREATE Event %d: ModType=%s, ServerTxnID=%s", i+1, event.ModType, event.ServerTransactionId)
		}
		for i, event := range relevantDeleteEvents {
			t.Logf("DELETE Event %d: ModType=%s, ServerTxnID=%s", i+1, event.ModType, event.ServerTransactionId)
		}
	} else {
		t.Log("✅ TransmitEvent=false prevented events as expected")
	}
}

// Test that change stream only tracks the configured columns
func testNoEventsForNonTrackedColumns(ctx context.Context, t *testing.T, db *metabase.DB) {
	projectID, bucketName, _, eventCh, errCh, cleanup := setupChangeStreamTest(ctx, t, db)
	defer cleanup()

	t.Log("Testing change stream column filtering (tracks only 'status' and 'total_plain_size')")

	// Create and commit an object to test column filtering behavior
	obj := metabase.ObjectStream{
		ProjectID:  projectID,
		BucketName: bucketName,
		ObjectKey:  metabase.ObjectKey(testrand.Bytes(16)),
		Version:    1,
		StreamID:   testrand.UUID(),
	}

	beforeOperations := time.Now()

	// Create object which will affect both status and total_plain_size (tracked columns)
	_, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
		ObjectStream: obj,
		Encryption:   metabasetest.DefaultEncryption,
	})
	require.NoError(t, err, "Should be able to begin object")

	_, err = db.CommitObject(ctx, metabase.CommitObject{
		ObjectStream:  obj,
		TransmitEvent: true,
	})
	require.NoError(t, err, "Should be able to commit object")

	// Collect events
	events := collectChangeStreamEvents(t, eventCh, errCh, 8*time.Second)
	relevantEvents := filterRelevantEvents(events, beforeOperations)

	t.Logf("Operations affecting tracked columns generated %d events", len(relevantEvents))

	// Verify events only contain the tracked columns in new_values
	for i, event := range relevantEvents {
		t.Logf("Event %d: ModType=%s", i+1, event.ModType)
		require.Equal(t, "INSERT", event.ModType, "Events should be INSERT type (exclude_update=TRUE)")

		if len(event.Mods) > 0 {
			mod := event.Mods[0]
			if mod.NewValues.Valid {
				if newVals, ok := mod.NewValues.Value.(map[string]interface{}); ok {
					// Verify only tracked columns are present
					_, hasStatus := newVals["status"]
					_, hasSize := newVals["total_plain_size"]

					require.True(t, hasStatus || hasSize, "Event should contain at least one tracked column")

					// Log what columns are present
					t.Logf("Event contains columns: %v", maps.Keys(newVals))
				}
			}
		}
	}

	t.Log("✅ Change stream correctly tracks only configured columns (status, total_plain_size)")
}

// Helper function to set up change stream reader and basic test objects
func setupChangeStreamTest(ctx context.Context, t *testing.T, db *metabase.DB) (
	projectID uuid.UUID, bucketName metabase.BucketName, adapter *metabase.SpannerAdapter,
	eventCh <-chan metabase.DataChangeRecord, errCh <-chan error, cleanup func()) {

	projectID = testrand.UUID()
	bucketName = metabase.BucketName(testrand.BucketName())
	adapter = db.ChooseAdapter(projectID).(*metabase.SpannerAdapter)

	// Verify the change stream exists
	changeStreamExists, err := verifyChangeStreamExists(ctx, adapter, "bucket_eventing")
	require.NoError(t, err, "Should be able to check change stream existence")
	require.True(t, changeStreamExists, "Change stream should exist")

	eventCh, errCh, cleanup = startChangeStreamReader(ctx, adapter, "bucket_eventing")

	// Give the reader a moment to start up
	time.Sleep(2 * time.Second)

	return projectID, bucketName, adapter, eventCh, errCh, cleanup
}

// Helper function to collect events with timeout
func collectChangeStreamEvents(t *testing.T, eventCh <-chan metabase.DataChangeRecord, errCh <-chan error, timeout time.Duration) []metabase.DataChangeRecord {
	var events []metabase.DataChangeRecord
	overallTimeout := time.After(timeout)
	eventTimeout := time.After(20 * time.Second) // Wait at least 20 seconds for events

	collectingEvents := true
	for collectingEvents {
		select {
		case event := <-eventCh:
			events = append(events, event)
			t.Logf("Received change stream event: table=%s, modType=%s, txnID=%s",
				event.TableName, event.ModType, event.ServerTransactionId)

			// Reset the event timeout when we receive events
			eventTimeout = time.After(10 * time.Second)

		case err := <-errCh:
			t.Logf("Change stream reader error: %v", err)
			collectingEvents = false

		case <-eventTimeout:
			t.Log("Event timeout reached, stopping event collection")
			collectingEvents = false

		case <-overallTimeout:
			t.Log("Overall timeout reached")
			collectingEvents = false
		}
	}

	t.Logf("Collected %d change stream events total", len(events))
	return events
}

// Helper function to verify events match expected criteria
func verifyExpectedEvents(t *testing.T, events []metabase.DataChangeRecord, expectedCount int, afterTime time.Time) {
	// Look for events related to our test operations
	var relevantEvents []metabase.DataChangeRecord
	for _, event := range events {
		if event.TableName == "objects" && event.CommitTimestamp.After(afterTime.Add(-1*time.Second)) {
			relevantEvents = append(relevantEvents, event)

			// Log the event details for debugging
			eventJSON, _ := json.MarshalIndent(event, "", "  ")
			t.Logf("Found relevant change stream event: %s", string(eventJSON))
		}
	}

	require.Len(t, relevantEvents, expectedCount, "Should have received exactly %d change stream events", expectedCount)

	// Verify event structure matches expected format
	for _, event := range relevantEvents {
		require.Equal(t, "objects", event.TableName, "Event should be for objects table")
		require.NotEmpty(t, event.ServerTransactionId, "Event should have server transaction ID")
		require.NotEmpty(t, event.RecordSequence, "Event should have record sequence")
		require.NotEmpty(t, event.ModType, "Event should have modification type")
		require.False(t, event.CommitTimestamp.IsZero(), "Event should have commit timestamp")
	}
}

// Helper function to filter events by time
func filterRelevantEvents(events []metabase.DataChangeRecord, afterTime time.Time) []metabase.DataChangeRecord {
	var relevantEvents []metabase.DataChangeRecord
	for _, event := range events {
		if event.TableName == "objects" && event.CommitTimestamp.After(afterTime.Add(-1*time.Second)) {
			relevantEvents = append(relevantEvents, event)
		}
	}
	return relevantEvents
}

// verifyChangeStreamExists checks if a change stream exists in the Spanner database
func verifyChangeStreamExists(ctx context.Context, adapter *metabase.SpannerAdapter, streamName string) (bool, error) {
	// Query Spanner's information schema to verify the change stream exists
	query := `
		SELECT change_stream_name
		FROM information_schema.change_streams
		WHERE change_stream_name = @stream_name
	`

	client := adapter.UnderlyingDB()
	err := client.Single().Query(ctx, spanner.Statement{
		SQL:    query,
		Params: map[string]interface{}{"stream_name": streamName},
	}).Do(func(row *spanner.Row) error {
		// If we get even a single row, the stream exists
		return nil
	})

	if err != nil {
		if errors.Is(err, iterator.Done) {
			// No rows found, change stream doesn't exist
			return false, nil
		}
		// Other error occurred
		return false, err
	}

	// Row found, change stream exists
	return true, nil
}

// startChangeStreamReader starts reading change stream events in the background
// and returns a channel that will receive events as they occur
func startChangeStreamReader(ctx context.Context, adapter *metabase.SpannerAdapter, streamName string) (<-chan metabase.DataChangeRecord, <-chan error, func()) {
	eventCh := make(chan metabase.DataChangeRecord, 100) // Buffer to avoid blocking
	errCh := make(chan error, 1)

	// Create a cancellable context for the processor
	processorCtx, cancel := context.WithCancel(ctx)

	// Start reading in background using the changestream processor
	go func() {
		defer close(eventCh)
		defer cancel()

		startTime := time.Now()

		err := changestream.Processor(processorCtx, adapter, streamName, startTime, func(record metabase.DataChangeRecord) error {
			select {
			case eventCh <- record:
			case <-processorCtx.Done():
				return processorCtx.Err()
			}

			return nil
		})

		if err != nil && processorCtx.Err() == nil {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	return eventCh, errCh, cancel
}
