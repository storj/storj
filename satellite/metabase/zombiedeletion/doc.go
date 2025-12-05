// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

// Package zombiedeletion provides automatic cleanup of "zombie" objects - pending objects
// from failed or abandoned uploads that were never committed or explicitly deleted.
//
// # Overview
//
// When an upload starts, metabase creates a pending object with a ZombieDeletionDeadline
// (typically 24 hours from creation). If the upload succeeds, the object transitions to
// committed status. However, if the client disconnects or crashes, the pending object remains
// in the database indefinitely, consuming storage space.
//
// This package implements a background chore that periodically scans for and deletes these
// zombie objects along with their associated segments.
//
// # What are Zombie Objects?
//
// A zombie object is a pending object that meets one of these conditions:
//  1. ZombieDeletionDeadline has passed (typically 24h after creation)
//  2. No upload activity for InactiveFor duration (default 24h)
//
// These objects will never be committed because:
//   - Client crashed during upload
//   - Network connection lost
//   - Client abandoned the upload
//   - Application bug left object in pending state
//
// # Architecture
//
// The Chore struct (chore.go:36) implements a background service that:
//  1. Runs periodically (default 15h production, 10s dev)
//  2. Queries metabase for zombie objects
//  3. Deletes found objects and their segments in batches
//  4. Logs cleanup activity
//
// # Configuration
//
// Config (chore.go:25) controls cleanup behavior:
//   - Interval: How often to run cleanup (default 15h production, 10s dev)
//   - Enabled: Toggle cleanup on/off (default true)
//   - ListLimit: Batch size for queries (default 100 objects)
//   - InactiveFor: Delete objects inactive for this duration (default 24h)
//   - AsOfSystemInterval: Staleness tolerance for reads (default -5m)
//
// # Cleanup Process
//
// The deleteZombieObjects function (chore.go:79) performs cleanup:
//  1. Calculates deadlines:
//     - DeadlineBefore: Current time (finds objects past ZombieDeletionDeadline)
//     - InactiveDeadline: Current time minus InactiveFor (finds stale uploads)
//  2. Calls metabase.DeleteZombieObjects with batch parameters
//  3. Metabase deletes matching objects and their segments
//  4. Process repeats until no more zombies found
//
// # Database Interaction
//
// Uses metabase.DeleteZombieObjects which:
//   - Queries objects with status=Pending and (ZombieDeletionDeadline < DeadlineBefore OR created_at < InactiveDeadline)
//   - Processes in batches to avoid long-running transactions
//   - Deletes segments first, then objects (maintains referential integrity)
//   - Uses AS OF SYSTEM TIME for eventual consistency
//
// # Why This Matters
//
// Without zombie cleanup:
//   - Database grows unbounded with failed uploads
//   - Query performance degrades
//   - Storage costs increase unnecessarily
//   - Backup/restore operations take longer
//
// Production satellites handle millions of uploads. Even a small percentage of failures
// would accumulate thousands of zombie objects per day.
//
// # Usage Example
//
// Typical initialization in satellite peer:
//
//	config := zombiedeletion.Config{
//	    Interval:    15 * time.Hour,
//	    Enabled:     true,
//	    ListLimit:   100,
//	    InactiveFor: 24 * time.Hour,
//	}
//	chore := zombiedeletion.NewChore(log, config, metabaseDB)
//	group.Go(func() error { return chore.Run(ctx) })
//
// # Testing
//
// TestingSetNow (chore.go:75) allows tests to control time:
//
//	chore.TestingSetNow(func() time.Time { return fixedTime })
//	// Now cleanup will use fixedTime instead of time.Now()
//
// This enables deterministic testing of deadline calculations.
//
// # Monitoring
//
// Uses monkit instrumentation (mon.Task) for metrics:
//   - Execution time per cleanup cycle
//   - Error rates
//   - Number of objects deleted (tracked by metabase)
//
// # Performance Considerations
//
// The chore is designed for efficiency:
//   - Batch processing prevents memory exhaustion
//   - AS OF SYSTEM TIME reduces lock contention
//   - Long interval (15h) spreads load over time
//   - Runs during low-traffic periods in production
//
// # Related Components
//
//   - metabase: Core database operations (DeleteZombieObjects)
//   - satellite/metainfo: Sets ZombieDeletionDeadline when creating objects
//   - sync2.Cycle: Periodic execution framework
//
// # Common Issues
//
// If zombie objects accumulate:
//  1. Check chore.config.Enabled is true
//  2. Verify chore is running in satellite peer
//  3. Check logs for errors during cleanup
//  4. Ensure metabase.DeleteZombieObjects is working correctly
//  5. Consider reducing InactiveFor duration
//
// If cleanup is too aggressive:
//  1. Increase InactiveFor duration
//  2. Increase Interval between cleanup cycles
//  3. Check that upload clients are properly setting deadlines
//
// # Making Changes
//
// When modifying zombie deletion:
//   - Test with various deadline scenarios
//   - Verify batch processing handles large volumes
//   - Check all database adapters (PostgreSQL, CockroachDB, Spanner)
//   - Monitor impact on database load during cleanup
//   - Ensure segments are deleted before objects (foreign key constraints)
package zombiedeletion
