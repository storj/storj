// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package rangedloop provides a parallel processing framework for iterating over all segments
// in the metabase, enabling efficient large-scale background operations.
//
// # Overview
//
// The ranged loop solves a fundamental problem: how to efficiently process millions or billions
// of segments stored in the metabase. Instead of sequentially scanning segments (which would take
// hours), the ranged loop splits the segment space into ranges and processes them in parallel.
//
// This architecture is used by critical satellite subsystems:
//   - Repair checker: Scans for under-replicated segments
//   - Garbage collection: Generates bloom filters of retained pieces
//   - Accounting tally: Counts stored data for billing
//   - Audit: Selects segments for verification
//
// The ranged loop reduces processing substantially by parallelizing work across
// multiple goroutines and database connections.
//
// # Core Concepts
//
// Range Splitting:
//   - Segment space divided by StreamID (UUID) ranges
//   - Each range processed independently in parallel
//   - Configurable parallelism (default: 2 concurrent ranges)
//
// Map-Reduce Pattern:
//   - Map: Each range processes its segments independently (Fork → Process)
//   - Reduce: Results merged back together (Join)
//   - Enables parallel processing with coordinated final results
//
// Observer Pattern:
//   - Multiple observers can subscribe to same loop
//   - Each observer processes all segments independently
//   - Observers don't interfere with each other
//
// # Architecture
//
// Service (service.go:44):
//   - Orchestrates the entire loop execution
//   - Manages observer lifecycle
//   - Coordinates parallel range processing
//   - Runs periodically (default: 2h interval)
//
// Observer (observer.go:43):
//   - Interface for segment processing logic
//   - Implements map-reduce pattern
//   - Methods: Start, Fork, Join, Finish
//
// Partial (observer.go:61):
//   - Processes subset of segments for one range
//   - Created by Fork, merged by Join
//   - Process() called repeatedly with segment batches
//
// RangeSplitter (provider.go):
//   - Creates segment ranges for parallel processing
//   - Returns SegmentProviders for each range
//   - Implementations: database-backed, Avro-backed
//
// SegmentProvider (provider.go):
//   - Iterates segments within a single range
//   - Calls callback with batches of segments
//   - Handles pagination and error recovery
//
// # Observer Lifecycle
//
// For each loop iteration, observers go through these phases:
//
// 1. Start(ctx, time.Time):
//   - Called once at beginning of loop
//   - Initialize state, open resources
//   - Return error to skip this iteration
//
// 2. Fork(ctx) → Partial:
//   - Called once per range (parallelism times)
//   - Create independent partial observer
//   - Each partial processes subset of segments
//   - NOT called concurrently
//
// 3. Process(ctx, []Segment):
//   - Called on Partial repeatedly with batches
//   - Each batch contains up to BatchSize segments
//   - Process segments and accumulate results
//   - NOT called concurrently on same Partial
//   - CAN be called concurrently on different Partials
//
// 4. Join(ctx, Partial):
//   - Called once per Partial after range completes
//   - Merge Partial results into main observer
//   - Reduce step of map-reduce pattern
//   - NOT called concurrently
//
// 5. Finish(ctx):
//   - Called once after all ranges processed
//   - Finalize results, close resources
//   - Last chance to report errors
//
// Example flow with parallelism=2:
//
//	observer.Start(ctx, now)
//	partial1 := observer.Fork(ctx)  // For range 1
//	partial2 := observer.Fork(ctx)  // For range 2
//
//	// Parallel processing (goroutines)
//	partial1.Process(ctx, batch1a)
//	partial2.Process(ctx, batch2a)
//	partial1.Process(ctx, batch1b)
//	partial2.Process(ctx, batch2b)
//	// ... more batches ...
//
//	// Sequential joining (back on main thread)
//	observer.Join(ctx, partial1)
//	observer.Join(ctx, partial2)
//	observer.Finish(ctx)
//
// # Configuration
//
// Config (service.go:29):
//   - Parallelism: Number of concurrent ranges (default: 2)
//   - BatchSize: Segments per batch (default: 2500)
//   - AsOfSystemInterval: Staleness tolerance for reads (default: -5m)
//   - Interval: Loop frequency (default: 2h production, 10s dev)
//   - SpannerStaleInterval: Spanner-specific staleness (default: 0)
//   - SuspiciousProcessedRatio: Detect anomalies in processing (default: 0.03)
//
// Higher parallelism:
//   - Faster processing
//   - More database connections
//   - More memory usage
//   - Diminishing returns beyond CPU count
//
// Larger batch size:
//   - Fewer database round trips
//   - Better throughput
//   - More memory per batch
//   - Longer time per Process() call
//
// # Implementing an Observer
//
// Step 1: Define Observer and Partial types
//
//	type MyObserver struct {
//	    db *metabase.DB
//	    // Accumulated results
//	    totalCount int64
//	    mu sync.Mutex
//	}
//
//	type MyPartial struct {
//	    // Per-range state
//	    count int64
//	}
//
// Step 2: Implement Observer interface
//
//	func (o *MyObserver) Start(ctx context.Context, t time.Time) error {
//	    o.mu.Lock()
//	    o.totalCount = 0
//	    o.mu.Unlock()
//	    return nil
//	}
//
//	func (o *MyObserver) Fork(ctx context.Context) (rangedloop.Partial, error) {
//	    return &MyPartial{}, nil
//	}
//
//	func (o *MyObserver) Join(ctx context.Context, partial rangedloop.Partial) error {
//	    p := partial.(*MyPartial)
//	    o.mu.Lock()
//	    o.totalCount += p.count
//	    o.mu.Unlock()
//	    return nil
//	}
//
//	func (o *MyObserver) Finish(ctx context.Context) error {
//	    log.Info("processed", zap.Int64("count", o.totalCount))
//	    return nil
//	}
//
// Step 3: Implement Partial interface
//
//	func (p *MyPartial) Process(ctx context.Context, segments []rangedloop.Segment) error {
//	    for _, segment := range segments {
//	        // Process each segment
//	        if !segment.Inline() {
//	            p.count++
//	        }
//	    }
//	    return nil
//	}
//
// Step 4: Register observer with service
//
//	observer := &MyObserver{db: db}
//	service := rangedloop.NewService(log, config, provider, []rangedloop.Observer{observer})
//	group.Go(func() error { return service.Run(ctx) })
//
// # Thread Safety
//
// Observer methods NOT called concurrently:
//   - Start()
//   - Fork()
//   - Join()
//   - Finish()
//
// Partial.Process() NOT called concurrently on SAME Partial:
//   - Safe to maintain mutable state in Partial
//   - No locking needed within Partial
//
// Partial.Process() CAN be called concurrently on DIFFERENT Partials:
//   - Must not share mutable state between Partials
//   - Join() is responsible for thread-safe merging
//
// Observer state accessed by Join() must be protected:
//   - Use mutex for totalCount, maps, etc.
//   - Or use atomic operations
//   - Or use channels for communication
//
// # Segment Type
//
// Segment (observer.go:16):
//   - Alias for metabase.LoopSegmentEntry
//   - Contains: StreamID, Position, CreatedAt, ExpiresAt, RepairedAt
//   - Contains: RootPieceID, EncryptedSize, PlainOffset, PlainSize
//   - Contains: Pieces (node locations), Redundancy, Placement
//
// Helper methods:
//   - Inline(): Returns true if segment stored inline (observer.go:19)
//   - Expired(now): Returns true if segment expired (observer.go:24)
//   - PieceSize(): Calculates piece size for erasure coding (observer.go:29)
//
// # Range Splitting Strategies
//
// Database-backed (providerdb.go):
//   - Queries metabase segments table
//   - Splits by StreamID ranges
//   - Uses AS OF SYSTEM TIME for consistency
//   - Default implementation for production
//
// Avro-backed (provider_avro.go):
//   - Reads segments from Avro files
//   - Useful for offline processing
//   - Integrates with avrometabase package
//
// # Error Handling
//
// Observer errors:
//   - Start() error: Observer skipped for this iteration
//   - Fork() error: Observer skipped for this iteration
//   - Process() error: Range skipped, other ranges continue
//   - Join() error: Recorded, but other observers continue
//   - Finish() error: Recorded, but other observers continue
//
// Service continues despite observer errors:
//   - One observer failing doesn't stop others
//   - Errors logged and reported in ObserverDuration
//   - Duration set to -1 when observer errors
//
// Context cancellation:
//   - Propagated to all observers
//   - Processing stops gracefully
//   - Partial results may be incomplete
//
// # Performance Characteristics
//
// Typical production metrics:
//   - 1 billion segments
//   - Parallelism: 4
//   - BatchSize: 2500
//   - Processing time: 15-30 minutes
//
// Bottlenecks:
//   - Database query speed (segment table scan)
//   - Observer processing time per segment
//   - Network latency to database
//   - Memory allocation for batches
//
// Optimization tips:
//   - Increase parallelism (up to CPU count)
//   - Increase batch size (up to memory limits)
//   - Use AS OF SYSTEM TIME to reduce locking
//   - Minimize work in Process() - defer heavy work if possible
//   - Avoid allocations in hot paths
//
// # Monitoring
//
// Metrics via monkit:
//   - rangedloop.RunOnce: Execution time per iteration
//   - rangedloop.Observer.Process: Time per batch
//   - rangedloop.error: Error events
//
// ObserverDuration return value:
//   - Duration per observer
//   - -1 indicates observer error
//   - Use for performance tracking
//
// # Testing
//
// See rangedlooptest package for testing utilities:
//   - Mock segment providers
//   - Callback observers for simple testing
//   - Count observers for verification
//
// Example test:
//
//	func TestMyObserver(t *testing.T) {
//	    observer := &MyObserver{}
//	    provider := rangedlooptest.NewMockProvider(testSegments)
//	    service := rangedloop.NewService(log, config, provider, []rangedloop.Observer{observer})
//
//	    durations, err := service.RunOnce(ctx)
//	    require.NoError(t, err)
//	    require.Equal(t, expectedCount, observer.totalCount)
//	}
//
// # Common Patterns
//
// Pattern 1: Counting/Aggregation
//   - Each Partial accumulates counts
//   - Join() sums counts into observer
//   - Finish() reports total
//
// Pattern 2: Batch Updates
//   - Each Partial collects items to update
//   - Join() merges item lists
//   - Finish() performs batch database update
//
// Pattern 3: Filtering
//   - Process() filters segments by criteria
//   - Pass filtered segments to downstream processing
//   - Example: repair checker filters by health
//
// Pattern 4: Sampling
//   - Process() randomly samples segments
//   - Join() combines samples
//   - Example: audit selector picks random segments
//
// # Integration with Satellite
//
// Typical satellite setup:
//
//	repairObserver := repair.NewObserver(...)
//	gcObserver := gc.NewObserver(...)
//	auditObserver := audit.NewObserver(...)
//
//	service := rangedloop.NewService(
//	    log,
//	    config.RangedLoop,
//	    metabaseProvider,
//	    []rangedloop.Observer{repairObserver, gcObserver, auditObserver},
//	)
//
//	peer.Services.Add(lifecycle.Item{
//	    Name: "rangedloop",
//	    Run:  service.Run,
//	    Close: service.Close,
//	})
//
// Multiple observers share the same loop iteration, reducing database load.
//
// # Making Changes
//
// When implementing observers:
//   - Ensure Fork() creates independent Partials
//   - Make Join() thread-safe (use mutex)
//   - Handle nil/empty segment batches
//   - Return errors for unrecoverable failures
//   - Use monkit instrumentation
//
// When modifying the service:
//   - Maintain backward compatibility with existing observers
//   - Test with multiple observers simultaneously
//   - Verify error handling doesn't stop other observers
//   - Check performance with production-scale data
//
// # Related Packages
//
//   - metabase: Core segment types and database operations
//   - rangedloop/rangedlooptest: Testing utilities
//   - avrometabase: Avro-backed segment provider
//   - satellite/repair: Example observer (repair checker)
//   - satellite/gc: Example observer (garbage collection)
//   - satellite/audit: Example observer (audit selector)
package rangedloop
