// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package changestream provides Spanner change stream processing for real-time
// metabase data change capture.
//
// # Overview
//
// Google Cloud Spanner change streams track data modifications in real-time.
// This package consumes those streams and delivers DataChangeRecord events to
// a caller-supplied callback, handling all partition lifecycle management
// internally.
//
// The main entry point is Processor:
//
//	err := changestream.Processor(ctx, log, adapter, "my_stream",
//	    startTime, func(record changestream.DataChangeRecord) error {
//	        // Handle object change
//	        return nil
//	    })
//
// # Architecture
//
// Spanner change streams are partitioned. Each partition is an independent
// cursor that must be read concurrently. Partitions split over time as Spanner
// rebalances load, producing child partitions that must be tracked and started.
//
// The Processor function manages this lifecycle:
//   - Stores partition state in a Spanner metadata table (<feedName>_metadata)
//   - Runs a main loop (processLoop) that schedules and starts partition goroutines
//   - Each partition goroutine calls ReadPartition, which streams ChangeRecords
//   - A MetadataBatcher collects watermark/state/child-partition writes and
//     flushes them in batched Spanner transactions to reduce round-trips
//
// # Partition States
//
// Each partition progresses through states stored in the metadata table:
//
//	StateCreated → StateScheduled → StateRunning → StateFinished
//
// StateCreated: partition discovered (child of a split), not yet ready to run.
// StateScheduled: all parent partitions are finished; ready to start.
// StateRunning: a goroutine is actively reading this partition.
// StateFinished: the partition's change stream cursor has been exhausted.
//
// # Core Types
//
// ChangeRecord is the raw record returned by Spanner for each row of the
// change stream query. Exactly one of its fields is non-empty per row:
//   - DataChangeRecord: an INSERT, UPDATE, or DELETE on a watched table
//   - HeartbeatRecord: a keep-alive signal during idle periods
//   - ChildPartitionsRecord: notification that this partition has split
//
// DataChangeRecord contains:
//   - CommitTimestamp: when the change was committed
//   - TableName: which table was modified
//   - ModType: "INSERT", "UPDATE", or "DELETE"
//   - Mods: row modifications (keys, new values, old values as JSON)
//   - ColumnTypes: schema metadata for affected columns
//
// ChildPartitionsRecord contains:
//   - StartTimestamp: when the child partitions become active
//   - ChildPartitions: list of child partition tokens and their parent tokens
//
// # Adapter Interface
//
// Adapter abstracts the Spanner operations needed by the processor:
//   - ReadChangeStreamPartition: streams ChangeRecords for one partition
//   - ChangeStreamNoPartitionMetadata: checks if the metadata table is empty
//   - GetChangeStreamPartitionsByState: queries partitions by state
//   - ScheduleChangeStreamPartitions: promotes ready Created partitions to Scheduled
//   - UpdateChangeStreamPartitions: applies batched metadata writes
//
// # Metadata Batcher
//
// MetadataBatcher reduces Spanner round-trips by buffering three categories
// of writes under a mutex:
//   - Watermark updates (last-write-wins per partition token)
//   - State transitions (StateRunning, StateFinished, etc.)
//   - New child partition inserts
//
// Flush writes all buffered updates in a single Spanner transaction.
// The main loop flushes on a 1-second ticker and immediately before calling
// ScheduleChangeStreamPartitions (so StateFinished is visible to the query).
//
// # SQL Query
//
// ReadPartition uses Spanner's table-valued function syntax:
//
//	SELECT ChangeRecord FROM READ_<stream_name>(
//	    start_timestamp => @start_time,
//	    partition_token => @partition_token,
//	    heartbeat_milliseconds => 60000
//	)
//
// This query is long-running and streams results as changes occur.
// All change stream reads run at PRIORITY_LOW to minimize production impact.
//
// # Partition Splitting
//
// When Spanner splits a partition:
//  1. ReadPartition receives a ChildPartitionsRecord
//  2. processPartition calls batcher.AddChildPartition for each child
//  3. The main loop flushes and calls ScheduleChangeStreamPartitions
//  4. Children whose parents are all finished move to StateScheduled
//  5. The main loop starts a new goroutine for each scheduled partition
//  6. The parent partition's cursor ends; processPartition marks it StateFinished
//
// # Testing
//
// The Adapter interface includes test helpers for creating and tearing down
// change streams and their metadata tables against the Spanner emulator:
//
//	adapter.TestCreateChangeStream(ctx, "test_stream")
//	defer adapter.TestDeleteChangeStream(ctx, "test_stream")
package changestream
