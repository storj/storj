// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package rangedlooptest provides testing utilities for the ranged loop package,
// including mock providers, simple observers, and helpers for testing ranged loop functionality.
//
// # Overview
//
// This package simplifies testing ranged loop observers and the loop service itself by providing:
//   - In-memory segment providers (no database needed)
//   - Simple observer implementations for common test scenarios
//   - Utilities for testing concurrency and timing
//
// Instead of setting up a full metabase database for tests, you can use these utilities
// to test observer logic in isolation.
//
// # Mock Providers
//
// RangeSplitter (provider.go:17):
//   - In-memory implementation of rangedloop.RangeSplitter
//   - Stores segments in []rangedloop.Segment slice
//   - Splits segments into ranges for testing parallelism
//   - No database connection required
//
// Usage:
//
//	segments := []rangedloop.Segment{
//	    {StreamID: uuid1, Position: metabase.SegmentPosition{Index: 0}, ...},
//	    {StreamID: uuid1, Position: metabase.SegmentPosition{Index: 1}, ...},
//	    {StreamID: uuid2, Position: metabase.SegmentPosition{Index: 0}, ...},
//	}
//	splitter := &rangedlooptest.RangeSplitter{Segments: segments}
//
//	// Use with ranged loop service
//	service := rangedloop.NewService(log, config, splitter, observers)
//	durations, err := service.RunOnce(ctx)
//
// The RangeSplitter ensures segments from the same stream are handled by the same
// SegmentProvider, maintaining the invariant that a stream's segments are processed
// together (provider.go:32-35).
//
// SegmentProvider (provider.go:24):
//   - In-memory implementation of rangedloop.SegmentProvider
//   - Iterates over subset of segments for one range
//   - Calls callback function with batches
//   - Respects configured batch size
//
// # Test Observers
//
// CallbackObserver (callbackobserver.go:18):
//   - Observer with configurable callbacks for each lifecycle method
//   - Useful for testing specific scenarios without full implementation
//   - Can act as both Observer and Partial (same instance)
//
// Fields:
//   - OnStart: func(context.Context, time.Time) error
//   - OnFork: func(context.Context) (rangedloop.Partial, error)
//   - OnJoin: func(context.Context, rangedloop.Partial) error
//   - OnFinish: func(context.Context) error
//   - OnProcess: func(context.Context, []rangedloop.Segment) error
//
// Usage:
//
//	processedSegments := []rangedloop.Segment{}
//	observer := &rangedlooptest.CallbackObserver{
//	    OnProcess: func(ctx context.Context, segments []rangedloop.Segment) error {
//	        processedSegments = append(processedSegments, segments...)
//	        return nil
//	    },
//	}
//
//	// Use with service
//	service := rangedloop.NewService(log, config, provider, []rangedloop.Observer{observer})
//	service.RunOnce(ctx)
//
//	// Verify all segments were processed
//	require.Len(t, processedSegments, expectedCount)
//
// The CallbackObserver includes small delays (callbackobserver.go:27-35) to ensure
// time measurements are visible, especially on Windows where time resolution is coarse.
//
// CountObserver (countobserver.go:17):
//   - Simple observer that counts total segments processed
//   - Useful for verifying loop processes all segments correctly
//   - Thread-safe via Fork/Join pattern
//
// Usage:
//
//	counter := &rangedlooptest.CountObserver{}
//	service := rangedloop.NewService(log, config, provider, []rangedloop.Observer{counter})
//	service.RunOnce(ctx)
//
//	require.Equal(t, expectedCount, counter.NumSegments)
//
// Implementation demonstrates proper Fork/Join pattern:
//   - Fork() returns new CountObserver instance (thread-safe)
//   - Each Partial counts segments in its range
//   - Join() adds partial counts to main observer
//
// SleepObserver (sleepobserver.go):
//   - Observer that sleeps during processing
//   - Useful for testing timing, concurrency, and cancellation
//   - Helps verify parallel processing is actually parallel
//
// Usage:
//
//	observer := &rangedlooptest.SleepObserver{
//	    Duration: 100 * time.Millisecond,
//	}
//	start := time.Now()
//	service.RunOnce(ctx)
//	duration := time.Since(start)
//
//	// With parallelism=2, should take ~50ms not 100ms
//	require.Less(t, duration, 75*time.Millisecond)
//
// # Infinite Provider
//
// InfiniteProvider (infiniteprovider.go):
//   - Generates segments indefinitely for load testing
//   - Useful for testing cancellation and resource limits
//   - Not recommended for normal tests (won't terminate naturally)
//
// Usage:
//
//	provider := rangedlooptest.NewInfiniteProvider()
//
//	// Test cancellation
//	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
//	defer cancel()
//	service.RunOnce(ctx) // Will be cancelled after 1 second
//
// # Testing Patterns
//
// Pattern 1: Basic Functionality Test
//
//	func TestMyObserver(t *testing.T) {
//	    segments := makeTestSegments(100)
//	    splitter := &rangedlooptest.RangeSplitter{Segments: segments}
//	    observer := &MyObserver{}
//
//	    service := rangedloop.NewService(log, config, splitter, []rangedloop.Observer{observer})
//	    durations, err := service.RunOnce(ctx)
//
//	    require.NoError(t, err)
//	    require.Equal(t, expectedResult, observer.Result)
//	}
//
// Pattern 2: Error Handling Test
//
//	func TestObserverError(t *testing.T) {
//	    observer := &rangedlooptest.CallbackObserver{
//	        OnProcess: func(ctx context.Context, segments []rangedloop.Segment) error {
//	            if len(segments) > 0 {
//	                return errors.New("test error")
//	            }
//	            return nil
//	        },
//	    }
//
//	    service := rangedloop.NewService(log, config, provider, []rangedloop.Observer{observer})
//	    durations, err := service.RunOnce(ctx)
//
//	    // Service continues despite observer error
//	    require.NoError(t, err)
//	    require.Equal(t, time.Duration(-1), durations[0].Duration) // -1 indicates error
//	}
//
// Pattern 3: Parallelism Test
//
//	func TestParallelProcessing(t *testing.T) {
//	    segments := makeTestSegments(1000)
//	    splitter := &rangedlooptest.RangeSplitter{Segments: segments}
//
//	    config := rangedloop.Config{
//	        Parallelism: 4,
//	        BatchSize: 100,
//	    }
//
//	    sleeper := &rangedlooptest.SleepObserver{Duration: 100 * time.Millisecond}
//	    counter := &rangedlooptest.CountObserver{}
//
//	    service := rangedloop.NewService(log, config, splitter,
//	        []rangedloop.Observer{sleeper, counter})
//
//	    start := time.Now()
//	    durations, err := service.RunOnce(ctx)
//	    elapsed := time.Since(start)
//
//	    require.NoError(t, err)
//	    require.Equal(t, 1000, counter.NumSegments)
//	    // With 4x parallelism, should be significantly faster than serial
//	}
//
// Pattern 4: Fork/Join Test
//
//	func TestForkJoin(t *testing.T) {
//	    segments := makeTestSegments(100)
//	    splitter := &rangedlooptest.RangeSplitter{Segments: segments}
//
//	    forkCount := 0
//	    joinCount := 0
//
//	    observer := &rangedlooptest.CallbackObserver{
//	        OnFork: func(ctx context.Context) (rangedloop.Partial, error) {
//	            forkCount++
//	            return &rangedlooptest.CountObserver{}, nil
//	        },
//	        OnJoin: func(ctx context.Context, partial rangedloop.Partial) error {
//	            joinCount++
//	            return nil
//	        },
//	    }
//
//	    config := rangedloop.Config{Parallelism: 3}
//	    service := rangedloop.NewService(log, config, splitter, []rangedloop.Observer{observer})
//	    service.RunOnce(ctx)
//
//	    require.Equal(t, 3, forkCount) // Fork called once per range
//	    require.Equal(t, 3, joinCount) // Join called once per partial
//	}
//
// # Stream Grouping
//
// The RangeSplitter ensures segments from the same stream stay together (provider.go:32-35):
//
//	// The segments for a given stream must be handled by a single segment
//	// provider. Split the segments into streams.
//	streams := streamsFromSegments(m.Segments)
//
// This is important because:
//   - Observers may assume stream segments are processed together
//   - Segment ordering within a stream may be significant
//   - Some operations require all segments of a stream
//
// The implementation:
//  1. Groups segments by StreamID (provider.go:70-89)
//  2. Splits streams (not individual segments) into ranges
//  3. Each SegmentProvider gets complete streams
//
// # Performance Considerations
//
// In-memory providers are fast:
//   - No database queries
//   - No network latency
//   - Ideal for unit tests
//
// However, they don't test:
//   - Database query performance
//   - Batch size optimization
//   - AS OF SYSTEM TIME behavior
//   - Network failures
//
// For integration tests, use real database-backed providers.
//
// # Creating Test Segments
//
// Helper function for generating test segments:
//
//	func makeTestSegments(count int) []rangedloop.Segment {
//	    segments := make([]rangedloop.Segment, count)
//	    for i := 0; i < count; i++ {
//	        segments[i] = rangedloop.Segment{
//	            StreamID: testrand.UUID(),
//	            Position: metabase.SegmentPosition{Index: uint32(i)},
//	            CreatedAt: time.Now(),
//	            // ... other fields ...
//	        }
//	    }
//	    return segments
//	}
//
// For realistic testing, include:
//   - Multiple segments per stream (test stream grouping)
//   - Expired segments (test expiration filtering)
//   - Inline segments (test inline detection)
//   - Various placements (test placement logic)
//
// # Thread Safety Testing
//
// To verify thread safety of observers:
//
//	func TestObserverThreadSafety(t *testing.T) {
//	    segments := makeTestSegments(10000)
//	    splitter := &rangedlooptest.RangeSplitter{Segments: segments}
//
//	    observer := &MyObserver{}
//	    config := rangedloop.Config{
//	        Parallelism: 10, // High parallelism to stress test
//	        BatchSize: 100,
//	    }
//
//	    service := rangedloop.NewService(log, config, splitter, []rangedloop.Observer{observer})
//
//	    // Run multiple times to detect race conditions
//	    for i := 0; i < 100; i++ {
//	        _, err := service.RunOnce(ctx)
//	        require.NoError(t, err)
//	    }
//
//	    // Verify results are consistent
//	    require.Equal(t, expectedResult, observer.Result)
//	}
//
// Run with `-race` flag to detect data races:
//
//	go test -race ./satellite/mypackage
//
// # Making Changes
//
// When adding new test utilities:
//   - Implement standard interfaces (Observer, Partial, RangeSplitter, etc.)
//   - Document expected behavior clearly
//   - Include usage examples in comments
//   - Test with various parallelism settings
//
// When modifying existing utilities:
//   - Maintain backward compatibility
//   - Update tests that use the utilities
//   - Document behavior changes
//
// # Related Packages
//
//   - rangedloop: Main package being tested
//   - metabase: Segment types and database operations
//   - testcontext: Test context management
//   - testrand: Random data generation for tests
//
// # Common Issues
//
// Test flakiness:
//   - Use sufficient delays in SleepObserver
//   - Account for timing variations in assertions
//   - Avoid hardcoded timing expectations
//
// Memory usage:
//   - Large segment slices can consume significant memory
//   - Use smaller counts for unit tests
//   - Clear segments after tests if needed
//
// Stream grouping violations:
//   - Verify segments from same stream stay together
//   - Test with multiple segments per stream
//   - Check that ordering is preserved within streams
package rangedlooptest
