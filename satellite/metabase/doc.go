// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package metabase provides the core metadata storage layer for Storj, managing all information
// about objects (files) and segments (pieces of files) stored in the network.
//
// # Architecture Overview
//
// Metabase acts as the single source of truth for what data exists in the Storj network. It supports
// multiple database backends (PostgreSQL, CockroachDB, Google Cloud Spanner) through an adapter pattern,
// allowing the same business logic to work across different database implementations.
//
// Key Components:
//   - DB: Main database handle with connection management (db.go:64)
//   - Adapter: Interface abstracting database operations (adapter.go:31)
//   - NodeAliasCache: Maps NodeIDs to compact aliases (aliascache.go:22)
//
// # Core Data Model
//
// The metabase stores two primary entity types:
//
// Objects (RawObject in raw.go:30):
//   - Represents a complete file in the network
//   - Key fields: ProjectID, BucketName, ObjectKey, Version
//   - Status: Pending, CommittedUnversioned, CommittedVersioned, DeleteMarkerVersioned, DeleteMarkerUnversioned
//   - Contains metadata: size, encryption, expiration, retention
//   - ZombieDeletionDeadline prevents abandoned uploads from bloating database
//
// Segments (RawSegment in raw.go:59):
//   - Represents (usually up to a 64MB but not always) piece of an object
//   - Contains piece locations (which storage nodes hold erasure-coded data)
//   - Position encoded as uint64: upper 32 bits = Part, lower 32 bits = Index
//   - Can be inline (small data stored directly) or remote (references to storage nodes)
//
// Constraints:
//   - It should not be possible to modify committed objects and delete markers (except for Object Lock metadata with proper authorization).
//   - It should not be possible to modify committed object segments.
//   - When committing a versioned object or delete marker with auto-assigned version, the new object should have the largest committed version.
//   - It should not be possible to modify or delete a locked object.
//   - An object and its segments should be atomically committed.
//   - There should only be one committed unversioned object or delete marker per object location (project_id, bucket_name, object_key).
//   - Segment positions must be unique within an object.
//   - StreamID must be unique across the system.
//
// # Database Schema
//
// Main tables:
//   - objects: Primary key (project_id, bucket_name, object_key, version)
//   - segments: Primary key (stream_id, position), foreign key to objects
//   - node_aliases: Maps NodeID (32 bytes) to NodeAlias (4 bytes) for space efficiency
//
// Migrations are managed per backend:
//   - PostgreSQL/CockroachDB: PostgresMigration() (db.go:331)
//   - Spanner: SpannerMigration() (db.go:692)
//
// # Supported Operations
//
// Object Lifecycle:
//   - BeginObjectNextVersion: Start new upload with auto-assigned version (commit.go:60)
//   - BeginObjectExactVersion: Start upload with specific version
//   - CommitObject: Finalize upload, transition from Pending to Committed
//   - GetObjectExactVersion: Fetch specific version (get.go:23)
//   - GetObjectLastCommitted: Fetch latest non-pending version (get.go:160)
//   - DeleteObjectExactVersion: Delete specific version with Object Lock support (delete.go:49)
//   - ListObjects: Iterate objects with optional prefix/delimiter (iterator.go)
//
// Segment Operations:
//   - BeginSegment: Reserve segment position
//   - CommitSegment: Write segment metadata with piece locations
//   - IterateLoopSegments: Efficiently scan all segments for background jobs (loop.go:81)
//
// # Database Backends
//
// PostgreSQL:
//   - Default development backend
//   - Full feature support
//   - Connection string: "postgres://..."
//
// CockroachDB:
//   - Production backend for distributed deployments
//   - Optimizations for CRDB-specific features
//   - Connection string: "cockroach://..."
//
// Spanner:
//   - Google Cloud Spanner for global distribution
//   - Supports change streams for CDC (see changestream package)
//   - Some DML limitations on primary keys
//   - Connection string: "spanner://..."
//
// # Multi-Adapter Support
//
// The DB can manage multiple adapters for different projects:
//   - ChooseAdapter(projectID) selects the appropriate backend (db.go:183)
//   - Useful for gradual migrations between database types
//   - Node alias coordination across adapters (first adapter is source of truth)
//
// # Node Aliases
//
// To reduce storage requirements, metabase uses 4-byte NodeAlias instead of 32-byte NodeID
// in the segments table:
//   - NodeAlias: int32 type (alias.go:21)
//   - NodeAliasCache: Write-through cache (aliascache.go:22)
//   - EnsureNodeAliases: Create aliases if they don't exist (alias.go:30)
//   - AliasPieces: Compressed piece representation (aliaspiece.go)
//
// This optimization significantly reduces segments table size in production.
//
// # Object Versioning
//
// Objects support versioning similar to S3:
//   - Version 0 (NextVersion): Auto-assigned next version
//   - Version >0: Explicit version number
//   - StreamVersionID: Public API combining Version (8 bytes) + StreamID suffix (8 bytes)
//   - Versioned buckets can have multiple versions per key
//   - Unversioned buckets have single version, new uploads replace old
//
// Delete markers (DeleteMarkerVersioned, DeleteMarkerUnversioned) represent soft deletions.
//
// # Object Lock and Retention
//
// Supports S3 Object Lock features:
//   - Retention: Time-based protection (governance or compliance mode)
//   - Legal Hold: Indefinite protection flag
//   - Governance mode can be bypassed with special permission
//   - Compliance mode cannot be bypassed
//   - Validated in BeginObjectNextVersion (commit.go:77) and DeleteObjectExactVersion (delete.go:54)
//
// # Zombie Objects
//
// Pending objects that are never committed become "zombies":
//   - ZombieDeletionDeadline set at object creation (default 24 hours)
//   - Background cleanup handled by zombiedeletion package
//   - Prevents database bloat from abandoned uploads
//
// # Integration with Satellite
//
// Other satellite components use metabase for:
//   - Metainfo service: Upload/download metadata operations
//   - Repair service: Find under-replicated segments via rangedloop
//   - Audit service: Select segments for verification
//   - Garbage collection: Generate bloom filters of retained pieces
//   - Accounting: Tally stored data for billing
//
// # Common Patterns
//
// Error Handling:
//   - Uses github.com/zeebo/errs for error classes
//   - Common errors: ErrObjectNotFound, ErrInvalidRequest, ErrConflict
//   - All errors wrapped with context
//
// Monitoring:
//   - All functions use defer mon.Task()(&ctx)(&err) for metrics
//
// Verification:
//   - Request structs have Verify() method validating fields
//
// Context Propagation:
//   - All functions take context.Context as first parameter
//
// # Testing
//
// Use metabasetest package for testing:
//   - Declarative test steps with Check() methods
//   - Helper functions: RandObjectStream(), CreateObject(), etc.
//   - TestingGetState() verifies complete database state (raw.go:114)
//   - TestingBatchInsertObjects/Segments for bulk test data (raw.go:296, raw.go:569)
//
// # Making Changes
//
// When modifying metabase code:
//  1. Test against all adapters (PostgreSQL, CockroachDB, Spanner)
//  2. Write reversible migrations
//  3. Consider query performance (metabase is frequently queried)
//  4. Maintain versioning logic carefully
//  5. Enforce Object Lock constraints
//  6. Always use node aliases in segments, never raw NodeIDs
//  7. Set ZombieDeletionDeadline when creating pending objects
//  8. Filter out pending and expired objects in queries
//
// # Key Files Reference
//
//   - db.go: DB type, Open(), adapter management
//   - adapter.go: Adapter interface, PostgresAdapter, CockroachAdapter
//   - common.go: Core types (ObjectLocation, ObjectStream, SegmentPosition, etc.)
//   - raw.go: RawObject, RawSegment, testing utilities
//   - commit.go: BeginObject, CommitObject operations
//   - get.go: GetObject operations
//   - delete.go: DeleteObject operations
//   - iterator.go: ListObjects implementation
//   - loop.go: IterateLoopSegments for background processing
//   - alias.go: Node alias operations
//   - aliascache.go: NodeAliasCache implementation
//
// # Related Packages
//
//   - metabase/zombiedeletion: Automatic cleanup of abandoned uploads
//   - metabase/rangedloop: Parallel segment processing framework
//   - metabase/avrometabase: Parse segments from Avro files
//   - metabase/changestream: Spanner change data capture
//   - metabase/metabasetest: Testing utilities
package metabase
