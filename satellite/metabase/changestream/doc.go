// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package changestream provides Spanner-specific change data capture (CDC) functionality
// for real-time processing of metabase data changes.
//
// # Overview
//
// Google Cloud Spanner supports change streams, a feature that tracks data modifications
// in real-time. This package provides utilities to:
//   - Listen to Spanner change streams
//   - Parse change records into structured data
//   - Process data changes via callbacks
//   - Handle partition splits for scalability
//
// Change streams enable event-driven architectures:
//   - Bucket eventing (S3-compatible event notifications)
//   - Real-time analytics
//   - Audit logging
//   - Cache invalidation
//   - Data replication
//
// # Spanner Change Streams Background
//
// Spanner change streams are table-level CDC features that:
//   - Track INSERT, UPDATE, DELETE operations
//   - Provide strong consistency guarantees
//   - Capture old and/or new values
//   - Scale horizontally via partitions
//   - Deliver changes in commit order
//
// Change streams are created via DDL:
//
//	CREATE CHANGE STREAM stream_name
//	FOR objects
//	OPTIONS (
//	  value_capture_type = 'NEW_ROW'
//	);
//
// This is handled by metabase migrations (db.go:723-742).
//
// # Core Types
//
// ChangeRecord (changestream.go:21):
//   - Container for change stream records
//   - Contains one of: DataChangeRecord, HeartbeatRecord, ChildPartitionsRecord
//   - Read from Spanner change stream query
//
// DataChangeRecord (changestream.go:48):
//   - Represents an actual data change (INSERT/UPDATE/DELETE)
//   - Key fields:
//   - CommitTimestamp: When change was committed
//   - TableName: Which table was modified
//   - ModType: "INSERT", "UPDATE", or "DELETE"
//   - Mods: Array of row modifications (keys, new values, old values)
//   - ColumnTypes: Schema information for affected columns
//   - Also contains transaction metadata: server_transaction_id, record_sequence
//
// HeartbeatRecord (changestream.go:65):
//   - Keep-alive signal from Spanner
//   - Ensures change stream query doesn't timeout during idle periods
//   - Contains only timestamp
//
// ChildPartitionsRecord (changestream.go:76):
//   - Notification of partition split
//   - Spanner dynamically splits partitions for load balancing
//   - Contains child partition tokens and parent partition tokens
//   - Consumer must start listening to child partitions
//
// # Reading Change Streams
//
// ReadPartitions function (changestream.go:101):
//   - Main entry point for consuming change stream
//   - Parameters:
//   - ctx: Context for cancellation
//   - log: Logger for diagnostics
//   - client: Spanner client
//   - name: Change stream name (e.g., "metabase_objects_stream")
//   - partitionToken: Specific partition to read (empty string = root)
//   - from: Start timestamp for reading changes
//   - callback: Function called for each DataChangeRecord
//   - Returns: Child partition records for dynamic scaling
//
// Usage:
//
//	childPartitions, err := changestream.ReadPartitions(
//	    ctx, log, client,
//	    "metabase_objects_stream",
//	    "", // Root partition
//	    time.Now().Add(-1*time.Hour),
//	    func(record changestream.DataChangeRecord) error {
//	        // Process the change
//	        log.Info("change", zap.String("table", record.TableName),
//	            zap.String("mod_type", record.ModType))
//	        return nil
//	    },
//	)
//
//	// If child partitions returned, listen to them too
//	for _, child := range childPartitions {
//	    for _, partition := range child.ChildPartitions {
//	        go changestream.ReadPartitions(ctx, log, client, "metabase_objects_stream",
//	            partition.Token, child.StartTimestamp, callback)
//	    }
//	}
//
// # SQL Query
//
// The ReadPartitions function uses Spanner's table-valued function:
//
//	SELECT ChangeRecord FROM READ_<stream_name>(
//	    start_timestamp => @start_time,
//	    heartbeat_milliseconds => 60000,
//	    partition_token => @partition_token  -- optional
//	)
//
// Parameters:
//   - start_timestamp: Begin reading from this time
//   - heartbeat_milliseconds: Heartbeat interval (default 60s)
//   - partition_token: Specific partition (omit for root partition)
//
// This query is long-running and streams results as changes occur.
//
// # Processing Change Records
//
// The ReadPartitions function iterates over results and:
//
// 1. Decodes ChangeRecord struct (changestream.go:117-122)
// 2. Dispatches based on record type:
//   - DataChangeRecord → Calls callback function (line 125-131)
//   - HeartbeatRecord → Logs and continues (line 133-135)
//   - ChildPartitionsRecord → Collects for return (line 137-142)
//
// Callback function signature:
//
//	func(record DataChangeRecord) error
//
// Return error to stop processing. Error is propagated to caller.
//
// # Data Change Structure
//
// Each DataChangeRecord contains:
//
// Identification:
//   - CommitTimestamp: When change was committed
//   - ServerTransactionId: Transaction identifier
//   - RecordSequence: Order within transaction
//   - TableName: Which table was modified
//
// Modification:
//   - ModType: "INSERT", "UPDATE", "DELETE"
//   - Mods: Array of modified rows
//   - Keys: Primary key values (JSON)
//   - NewValues: New column values (JSON)
//   - OldValues: Old column values (JSON, if captured)
//   - ValueCaptureType: "NEW_ROW", "OLD_VALUES", "NEW_ROW_AND_OLD_VALUES"
//
// Schema:
//   - ColumnTypes: Array of column metadata
//   - Name: Column name
//   - CodeType: Spanner type code
//   - IsPrimaryKey: Is this column part of primary key
//   - OrdinalPosition: Position in table schema
//
// Transaction:
//   - IsLastRecordInTransactionInPartition: Is this the last record for this transaction
//   - NumberOfRecordsInTransaction: Total records in transaction
//   - NumberOfPartitionsInTransaction: How many partitions this transaction spans
//   - TransactionTag: Optional transaction tag
//   - IsSystemTransaction: Is this a system-generated transaction
//
// # Value Capture Types
//
// Spanner change streams can capture different value sets:
//
// NEW_ROW (default):
//   - Mods.NewValues contains all columns
//   - Efficient for most use cases
//   - INSERT: new row values
//   - UPDATE: new row values (full row)
//   - DELETE: no values (only keys)
//
// OLD_VALUES:
//   - Mods.OldValues contains changed columns only
//   - UPDATE: only changed old values
//   - More efficient for wide tables
//
// NEW_ROW_AND_OLD_VALUES:
//   - Both new and old values captured
//   - Useful for auditing and diffing
//   - Higher storage cost
//
// # Partition Splitting
//
// Spanner dynamically splits change stream partitions for scalability:
//
// 1. Consumer reads from root partition (partitionToken = "")
// 2. Spanner decides to split partition (based on load)
// 3. ChildPartitionsRecord returned with:
//   - StartTimestamp: When child partitions become active
//   - ChildPartitions: Array of child partition tokens
//   - ParentPartitionTokens: Which parent partitions are being replaced
//
// 4. Consumer must start listening to child partitions
// 5. Consumer stops listening to parent partition
//
// Consumers must handle partition management:
//   - Track active partitions
//   - Start goroutines for new child partitions
//   - Stop reading from parent partitions
//   - Handle partition lifecycle
//
// # JSON Encoding
//
// Mods fields (Keys, NewValues, OldValues) are JSON-encoded:
//
//	// Example INSERT
//	{
//	  "keys": {"stream_id": "uuid-value", "position": 0},
//	  "new_values": {
//	    "stream_id": "uuid-value",
//	    "position": 0,
//	    "created_at": "2025-01-01T00:00:00Z",
//	    "encrypted_size": 65536,
//	    ...
//	  },
//	  "old_values": null
//	}
//
// Consumers must unmarshal JSON to extract field values.
//
// # Adapter Interface
//
// Adapter interface (changestream.go:83):
//   - Abstraction for change stream operations
//   - Methods:
//   - ChangeStream(): Read from change stream
//   - TestCreateChangeStream(): Create stream for testing
//   - TestDeleteChangeStream(): Delete stream after testing
//
// Allows mocking for tests and future implementations.
//
// # Error Handling
//
// ReadPartitions returns errors for:
//   - Spanner query failures
//   - Callback function errors
//   - Context cancellation
//   - Invalid change records
//
// Long-running reads:
//   - Change stream queries run indefinitely
//   - Use context with timeout or cancellation
//   - Handle temporary Spanner errors with retry
//
// Callback errors:
//   - Stop processing immediately
//   - Return error to caller
//   - Caller responsible for retry logic
//
// # Use Cases
//
// Bucket Eventing:
//   - Listen to objects table change stream
//   - Convert DataChangeRecord to S3 event format
//   - Publish to event bus (SQS, SNS, webhook)
//
// Real-time Analytics:
//   - Stream changes to analytics database
//   - Aggregate metrics in real-time
//   - Power dashboards and reports
//
// Audit Logging:
//   - Capture all data modifications
//   - Store in audit log table or external system
//   - Include transaction metadata and timestamps
//
// Cache Invalidation:
//   - Detect object/segment changes
//   - Invalidate corresponding cache entries
//   - Maintain cache consistency
//
// Data Replication:
//   - Replicate changes to another system
//   - Keep secondary database in sync
//   - Enable multi-region deployments
//
// # Performance Considerations
//
// Change stream overhead:
//   - Small impact on write performance (~5-10%)
//   - Storage cost for change stream retention
//   - Query consumes Spanner processing units
//
// Heartbeat interval:
//   - Default 60s prevents timeout during idle periods
//   - Lower values increase overhead
//   - Higher values risk timeout
//
// Partition count:
//   - More partitions = more parallelism
//   - Each partition requires separate goroutine/client
//   - Spanner automatically balances partitions
//
// Callback performance:
//   - Slow callbacks slow down change stream processing
//   - Consider async processing (channels/queues)
//   - Batch operations when possible
//
// # Testing
//
// TestCreateChangeStream (changestream.go:86):
//   - Creates change stream for testing
//   - Must be cleaned up with TestDeleteChangeStream
//
// Usage:
//
//	adapter.TestCreateChangeStream(ctx, "test_stream")
//	defer adapter.TestDeleteChangeStream(ctx, "test_stream")
//
//	// Test change stream functionality
//	changestream.ReadPartitions(ctx, log, client, "test_stream", ...)
//
// # Change Stream Lifecycle
//
// Creation (via migration):
//
//	CREATE CHANGE STREAM metabase_objects_stream
//	FOR objects
//	OPTIONS (value_capture_type = 'NEW_ROW');
//
// Reading:
//
//	changestream.ReadPartitions(ctx, log, client, "metabase_objects_stream", ...)
//
// Deletion (usually not needed in production):
//
//	DROP CHANGE STREAM metabase_objects_stream;
//
// # Integration with Metabase
//
// Metabase creates change stream during migration (db.go:723-742):
//
//	CREATE CHANGE STREAM objects_stream FOR objects
//
// Satellite components can then consume this stream:
//
//	client := metabaseDB.SpannerClient()
//	changestream.ReadPartitions(ctx, log, client, "objects_stream",
//	    "", time.Now(), func(record changestream.DataChangeRecord) error {
//	        // Handle object changes
//	        return nil
//	    })
//
// # Making Changes
//
// When modifying change stream handling:
//   - Test with partition splits (simulate load)
//   - Handle all record types (data, heartbeat, child partitions)
//   - Verify callback error handling
//   - Check performance with high change volume
//   - Test context cancellation
//
// When adding new change streams:
//   - Add to metabase migrations
//   - Choose appropriate value_capture_type
//   - Document retention period
//   - Plan for partition management
//
// # Related Packages
//
//   - metabase: Core metabase operations
//   - recordeddb: Spanner client wrapper
//   - satellite/console/bucketeventing: Example change stream consumer
//
// # Common Issues
//
// Partition management:
//   - Must listen to child partitions when split occurs
//   - Parent partition stops producing records after split
//   - Track active partitions to avoid missing changes
//
// Callback performance:
//   - Slow callbacks cause backpressure
//   - Consider async processing
//   - Monitor change stream lag
//
// Context cancellation:
//   - Long-running queries need proper cancellation
//   - Ensure goroutines exit cleanly
//   - Close resources properly
//
// Schema changes:
//   - ColumnTypes reflects current schema
//   - Old records may have different schemas
//   - Handle schema evolution gracefully
package changestream
