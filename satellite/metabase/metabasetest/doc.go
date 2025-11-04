// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package metabasetest provides testing utilities and helpers for metabase operations,
// enabling declarative test steps and simplified test data creation.
//
// # Overview
//
// This package simplifies testing metabase functionality by providing:
//  1. Declarative test step structs with Check() methods
//  2. Helper functions for creating test data (objects, segments)
//  3. Test runners for executing tests across multiple database backends
//  4. Default values for common test scenarios
//  5. Comparison and assertion utilities
//
// # Test Step Pattern
//
// The core pattern is declarative test steps that encapsulate:
//   - Input parameters (Opts)
//   - Expected outcomes (Version, ExpectVersion)
//   - Error expectations (ErrClass, ErrText)
//
// Each step has a Check() method that:
//  1. Executes the metabase operation
//  2. Validates error conditions
//  3. Asserts expected results
//  4. Returns the result for further testing
//
// Example (test.go:24):
//
//	BeginObjectNextVersion{
//	    Opts: metabase.BeginObjectNextVersion{
//	        ObjectStream: objectStream,
//	        Encryption:   DefaultEncryption,
//	    },
//	    Version: 1,
//	}.Check(ctx, t, db)
//
// This approach provides:
//   - Clear, readable test code
//   - Consistent error handling
//   - Automatic validation of results
//   - Easy test data setup
//
// # Available Test Steps
//
// Object Operations (test.go):
//   - BeginObjectNextVersion: Start new object with auto-assigned version (line 24)
//   - BeginObjectExactVersion: Start object with specific version (line 63)
//   - CommitObject: Finalize pending object (line 92)
//   - CommitObjectWithSegments: Commit with inline segment check (line 109)
//   - GetObjectExactVersion: Fetch specific version
//   - GetObjectLastCommitted: Fetch latest version
//   - DeleteObjectExactVersion: Delete specific version
//   - DeleteObjectLastCommitted: Delete latest version
//   - ListObjects: Iterate objects with prefix/delimiter
//
// Segment Operations (test.go):
//   - BeginSegment: Reserve segment position
//   - CommitSegment: Write segment metadata
//   - CommitSegmentPointer: Legacy segment commit
//   - CommitInlineSegment: Commit with inline data
//
// Batch Operations (test.go):
//   - BulkCommitObject: Commit multiple objects atomically
//
// Each test step validates:
//   - Successful execution returns expected data
//   - Failed execution returns expected error class and message
//   - Database state matches expectations
//   - Timestamps are within acceptable ranges
//
// # Helper Functions
//
// Random Data Generation (create.go:21-44):
//   - RandObjectStream(): Generate random object stream (line 22)
//   - RandObjectKey(): Generate random object key (line 33)
//   - RandEncryptedKeyAndNonce(): Generate segment metadata (line 38)
//
// Object Creation (create.go:46-106):
//   - CreatePendingObject(): Create pending object with N segments (line 46)
//   - CreateObject(): Create committed unversioned object (line 59)
//   - CreateObjectVersioned(): Create committed versioned object (line 70)
//   - CreateExpiredObject(): Create object with expiration (line 82)
//   - CreateObjectCopy(): Create object as copy of another
//   - CreateTestObject(): Generic test object creation
//
// Segment Creation (create.go):
//   - CreateSegments(): Create N segments for an object
//   - CreateSegment(): Create single segment
//
// These helpers handle boilerplate setup, allowing tests to focus on the scenario being tested.
//
// # Test Runners
//
// The package provides runners for executing tests across multiple database backends (run.go).
//
// RunWithConfig (run.go:43):
//   - Runs test function against all configured databases
//   - Supports config variations (timestamp versioning, old commit)
//   - Automatically creates and migrates test database
//   - Cleans up resources after test
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    metabasetest.RunWithConfig(t, metabase.Config{}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
//	        // Test code here
//	    })
//	}
//
// This automatically runs your test against:
//   - PostgreSQL (if configured)
//   - CockroachDB (if configured)
//   - Spanner (if configured)
//
// Config Variations (run.go:27-40):
//   - WithTimestampVersioning: Test with timestamp-based versioning (line 31)
//   - WithOldCommitObject: Test with legacy commit behavior (line 37)
//
// Variations allow testing the same scenario with different metabase configurations:
//
//	metabasetest.RunWithConfig(t, config, testFunc,
//	    metabasetest.WithTimestampVersioning,
//	    metabasetest.WithOldCommitObject,
//	)
//
// # Default Values
//
// Package provides sensible defaults for common test scenarios (defaults.go):
//   - DefaultEncryption: Standard encryption parameters
//   - DefaultRedundancy: Standard erasure coding scheme (80 pieces, 29 required)
//   - DefaultExpiration: Default object expiration time
//
// These reduce test boilerplate while remaining customizable when needed.
//
// # Object Lock Testing
//
// Dedicated helpers for Object Lock features (objectlock.go):
//   - CreateObjectWithRetention(): Create object with retention period
//   - CreateObjectWithLegalHold(): Create object with legal hold
//   - UpdateObjectRetention(): Modify retention settings
//   - TestRetentionCompliance(): Verify compliance mode enforcement
//
// # Invalid Input Testing
//
// Helpers for testing error conditions (invalid.go):
//   - InvalidObjectStream: Various invalid object stream scenarios
//   - InvalidEncryption: Invalid encryption parameters
//   - InvalidVersion: Invalid version numbers
//
// These ensure proper validation and error handling.
//
// # Common Testing Patterns
//
// Pattern 1: Simple Object Lifecycle
//
//	obj := metabasetest.RandObjectStream()
//	metabasetest.BeginObjectNextVersion{
//	    Opts: metabase.BeginObjectNextVersion{ObjectStream: obj},
//	    Version: 1,
//	}.Check(ctx, t, db)
//	metabasetest.CommitObject{
//	    Opts: metabase.CommitObject{ObjectStream: obj},
//	}.Check(ctx, t, db)
//
// Pattern 2: Error Testing
//
//	metabasetest.DeleteObjectExactVersion{
//	    Opts: metabase.DeleteObjectExactVersion{
//	        ObjectLocation: location,
//	        Version: 999, // Non-existent version
//	    },
//	    ErrClass: &metabase.ErrObjectNotFound,
//	}.Check(ctx, t, db)
//
// Pattern 3: Bulk Test Data
//
//	for i := 0; i < 100; i++ {
//	    obj := metabasetest.RandObjectStream()
//	    metabasetest.CreateObject(ctx, t, db, obj, 5)
//	}
//
// Pattern 4: State Verification
//
//	// Use metabase.TestingGetState() to inspect complete database state
//	state, err := db.TestingGetState(ctx)
//	require.NoError(t, err)
//	require.Len(t, state.Objects, expectedObjectCount)
//	require.Len(t, state.Segments, expectedSegmentCount)
//
// # Comparison Utilities
//
// Uses github.com/google/go-cmp for deep equality checks (common.go):
//   - Ignores timing variations (within 5 seconds)
//   - Handles slice ordering differences
//   - Compares complex nested structures
//
// This provides clear diff output when tests fail, showing exactly what differed.
//
// # Testing Best Practices
//
// 1. Use declarative steps for clarity:
//   - BeginObject, CommitObject, etc. steps are self-documenting
//   - Error expectations are explicit
//
// 2. Leverage helpers for test data:
//   - RandObjectStream() for unique test objects
//   - CreateObject() for setup
//
// 3. Test all database backends:
//   - Use RunWithConfig to ensure compatibility
//   - Test config variations when relevant
//
// 4. Verify error conditions:
//   - Always specify ErrClass for expected failures
//   - Optionally specify ErrText for exact message matching
//
// 5. Clean test data:
//   - testcontext handles cleanup automatically
//   - Tests run in isolated database instances
//
// 6. Check edge cases:
//   - Empty buckets
//   - Large object counts
//   - Concurrent operations
//   - Expired objects
//   - Versioned vs unversioned buckets
//
// # Integration with Metabase
//
// This package directly uses metabase operations:
//   - Calls db.BeginObjectNextVersion(), db.CommitObject(), etc.
//   - Returns actual metabase types (Object, Segment)
//   - Uses real database connections (not mocks)
//
// Tests using metabasetest are integration tests, not unit tests. They verify
// the complete database interaction, including SQL queries, transactions, and
// constraint enforcement.
//
// # Performance Considerations
//
// Test data creation can be slow for large datasets:
//   - CreateObject() writes to database synchronously
//   - Use TestingBatchInsertObjects() for bulk data (metabase/raw.go:296)
//   - Consider test parallelization with t.Parallel()
//
// For performance-sensitive tests:
//   - Minimize object/segment counts
//   - Reuse test data across sub-tests when safe
//   - Profile slow tests to identify bottlenecks
//
// # Making Changes
//
// When adding new metabase operations:
//  1. Add corresponding test step struct in test.go
//  2. Implement Check() method following existing patterns
//  3. Add helper function in create.go if commonly used
//  4. Document the new step in this file
//
// When modifying test helpers:
//   - Ensure backward compatibility with existing tests
//   - Update defaults.go if changing standard test values
//   - Run full test suite across all database backends
//
// # Related Packages
//
//   - metabase: Core package being tested
//   - testplanet: Higher-level integration testing (simulates full satellite)
//   - satellitedbtest: Database setup utilities
//   - testcontext: Test context and cleanup management
//
// # Common Issues
//
// Test flakiness:
//   - Timing assertions use 5-second tolerance (adjust if needed)
//   - Database state may differ between backends
//   - Ensure tests clean up properly
//
// Test failures on specific backend:
//   - Check for backend-specific SQL differences
//   - Verify migrations are identical across backends
//   - Look for timing-sensitive assumptions
//
// Slow tests:
//   - Reduce object/segment counts
//   - Use batch insert operations
//   - Run expensive tests only on one backend
package metabasetest
