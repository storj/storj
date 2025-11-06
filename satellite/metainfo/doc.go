// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package metainfo provides the gRPC API layer for object metadata operations in Storj,
// serving as the primary interface between uplinks (clients) and the satellite's metadata storage.
//
// # Architecture Overview
//
// Metainfo acts as the gateway to the Storj network, handling all client requests for object
// operations (upload, download, delete, copy, move) and coordinating between multiple satellite
// subsystems. It performs authentication, authorization, validation, rate limiting, and
// orchestrates the complex workflows required for distributed object storage.
//
// Key Responsibilities:
//   - Authenticate and authorize client requests via macaroon-based API keys
//   - Validate request parameters and enforce business rules
//   - Coordinate with metabase for metadata persistence
//   - Create bandwidth orders for storage node operations
//   - Enforce rate limits and usage quotas
//   - Track attribution for partner analytics
//   - Support advanced S3-compatible features (versioning, Object Lock)
//
// # Core Component
//
// Endpoint (endpoint.go):
//   - Main struct implementing pb.DRPCMetainfoUnimplementedServer
//   - Handles 50+ gRPC methods organized by domain
//   - 30+ dependencies injected at creation via NewEndpoint()
//
// # gRPC API Surface
//
// The Endpoint provides methods grouped by functionality:
//
// Object Operations (endpoint_object.go):
//   - BeginObject: Start new object upload with version assignment
//   - CommitObject: Finalize upload, transition pending to committed
//   - GetObject: Retrieve object metadata for downloads
//   - DownloadObject: Optimized metadata fetch with segment info
//   - ListObjects: Paginated object listing with prefix/delimiter
//   - ListPendingObjectStreams: List uncommitted uploads
//   - UpdateObjectMetadata: Modify Object Lock settings
//   - BeginMoveObject/FinishMoveObject: Server-side move
//   - BeginCopyObject/FinishCopyObject: Server-side copy
//   - GetObjectIPs: Retrieve storage node IPs for an object
//
// Segment Operations (endpoint_segment.go):
//   - BeginSegment: Select storage nodes, create upload order limits
//   - CommitSegment: Validate and store segment metadata with piece locations
//   - MakeInlineSegment: Store small segments directly in metadata
//   - DownloadSegment: Create download order limits for piece retrieval
//   - ListSegments: Iterate segments for a specific object
//   - RetryBeginSegmentPieces: Request replacement nodes for failed uploads
//   - DeletePart: Delete multipart upload part
//
// Delete Operations (endpoint_object_delete.go):
//   - BeginDeleteObject: Delete single object with Object Lock validation
//   - DeleteObjects: Batch delete multiple objects
//
// Bucket Operations (endpoint_bucket.go):
//   - CreateBucket: Create new bucket with optional settings
//   - GetBucket: Retrieve bucket metadata
//   - DeleteBucket: Delete empty bucket
//   - ListBuckets: List buckets in project
//   - GetBucketVersioning/SetBucketVersioning: Manage versioning state
//   - GetBucketLocation: Return bucket region
//   - GetBucketObjectLockConfiguration/SetBucketObjectLockConfiguration: Object Lock config
//   - GetBucketTagging/SetBucketTagging: Manage bucket tags
//
// Object Lock Operations (endpoint_object.go):
//   - GetObjectRetention/SetObjectRetention: Time-based retention management
//   - GetObjectLegalHold/SetObjectLegalHold: Legal hold management
//
// Batch Operations (batch.go):
//   - Batch: Execute multiple requests in single RPC call
//   - CompressedBatch: Batch with zstd compression support
//
// Utility Operations (endpoint.go):
//   - ProjectInfo: Get project metadata and limits
//   - RevokeAPIKey: Revoke compromised API key
//
// # Authentication and Authorization
//
// Authentication Flow (validation.go):
//  1. Extract macaroon-based API key from request header
//  2. Validate API key via console.APIKeys database
//  3. Check user account status (active, suspended, deleted)
//  4. Verify macaroon permissions match requested action
//  5. Check rate limits at project and object levels
//  6. Validate usage quotas via entitlements service
//
// API Key Types:
//   - Macaroon-based keys with caveats (restrictions)
//   - Supports action-based permissions: Read, Write, Delete, List
//   - Time-based expiration and path restrictions
//   - Revocation via revocation database
//
// # Rate Limiting
//
// The endpoint implements multi-level rate limiting:
//
// Project-Level (validation.go):
//   - Token bucket rate limiter per project
//   - Cached in LRU with expiration (config.RateLimiter.CacheCapacity)
//   - Default: 100 req/sec (RateLimiterConfig.Rate)
//
// Object-Level (validation.go):
//   - Upload limits: Per-object bloom filter + rate limiter (UploadLimiterConfig)
//   - Download limits: Per-object bloom filter + rate limiter (DownloadLimiterConfig)
//   - Space-efficient via bloomrate package
//
// User-Level:
//   - Enforced via entitlements service
//   - Storage, bandwidth, and segment limits
//
// # Data Flow Patterns
//
// Upload Workflow:
//  1. Client calls BeginObject → creates pending object in metabase
//  2. For each segment:
//     a. Client calls BeginSegment → endpoint selects storage nodes via overlay
//     b. Endpoint creates signed order limits via orders.Service
//     c. Client uploads pieces to storage nodes using orders
//     d. Client calls CommitSegment → endpoint validates pieces, stores segment metadata
//  3. Client calls CommitObject → endpoint finalizes object (pending → committed)
//
// Download Workflow:
//  1. Client calls DownloadObject → endpoint returns object + first segment metadata
//  2. For additional segments:
//     a. Client calls DownloadSegment → endpoint creates signed download orders
//  3. Client downloads pieces from storage nodes using orders
//  4. Client performs erasure decoding locally
//
// Delete Workflow:
//  1. Client calls BeginDeleteObject → endpoint validates Object Lock constraints
//  2. Endpoint deletes from metabase (cascades to segments)
//  3. Endpoint updates accounting tallies
//  4. Background GC sends bloom filters to storage nodes for piece cleanup
//
// Server-Side Copy Workflow:
//  1. Client calls BeginCopyObject → endpoint validates source and destination
//  2. Endpoint copies segment metadata in metabase (no piece movement)
//  3. Client calls FinishCopyObject → endpoint commits new object
//  4. Result: Two objects reference same pieces on storage nodes
//
// # Configuration
//
// Config struct (config.go) contains 40+ configuration fields:
//
// Core Settings:
//   - RS: Erasure coding parameters (k/m/o/n-sharesize format)
//   - MaxInlineSegmentSize: Threshold for inline vs remote segments
//   - MaxSegmentSize: Maximum segment size (typically 64 MiB)
//   - MaxMetadataSize: Maximum metadata per object
//   - MaxCommitInterval: Timeout for upload completion
//   - MinRemoteSegmentSize: Minimum size for remote segments
//
// Feature Flags:
//   - ObjectLockEnabled: Enable Object Lock functionality
//   - UseBucketLevelObjectVersioning: Bucket-level versioning control
//   - UseBucketLevelObjectLockByProjectID: Per-project Object Lock
//   - ServerSideCopy: Enable server-side copy optimization
//   - ServerSideCopyDisabled: Disable server-side copy for specific projects
//
// Rate Limiting:
//   - RateLimiter: Project-level rate limiting
//   - UploadLimiterConfig: Per-object upload limits
//   - DownloadLimiterConfig: Per-object download limits
//
// Testing:
//   - TestingMigrationMode: Read-only mode for migrations
//   - TestingSpannerProjects: Route specific projects to Spanner
//   - TestingTimestampVersioning: Use timestamps instead of version numbers
//   - TestingTwoRoundtripCommit: Enable new commit protocol
//
// # Dependencies
//
// The Endpoint coordinates with multiple satellite subsystems:
//
// Core Storage:
//   - metabase.DB: Metadata persistence (objects, segments)
//   - buckets.Service: Bucket management and settings
//
// Network Coordination:
//   - overlay.Service: Storage node selection and reputation
//   - orders.Service: Bandwidth order creation and validation
//
// User Management:
//   - console.APIKeys: API key validation
//   - console.Projects: Project metadata and limits
//   - console.Users: User account status
//
// Accounting:
//   - accounting.Service: Usage tracking and tallying
//   - entitlements.Service: Limit enforcement
//
// Security:
//   - revocation.DB: API key revocation
//   - signing.Signer: Cryptographic signing for stream/segment IDs
//   - pointerverification.Service: Validates piece uploads
//
// Analytics:
//   - attribution.DB: Partner attribution tracking
//   - console.APIKeyTails: API key usage analytics
//
// Performance:
//   - SuccessTrackers: Node reliability tracking for selection optimization
//   - FailureTracker: Track and rate-limit problematic nodes
//
// # Key Patterns
//
// Monolithic Endpoint:
//   - Single Endpoint struct handles all gRPC methods
//   - Methods organized across multiple files by domain
//   - Heavy dependency injection (30+ dependencies)
//
// Error Handling:
//   - ConvertMetabaseErr(): Translates metabase errors to gRPC status codes (endpoint.go)
//   - ConvertKnownErr(): Handles known domain errors (endpoint.go)
//   - Always returns rpcstatus errors to clients
//   - Structured logging via zap for debugging
//
// Cryptographic Signing:
//   - SignStreamID()/VerifyStreamID(): Prevents tampering with upload state (signing.go)
//   - SignSegmentID()/VerifySegmentID(): Protects segment references (signing.go)
//   - HMAC-SHA256 with satellite private key
//
// Caching:
//   - LRU cache for rate limiters (limiterCache)
//   - LRU cache for user info (userInfoCache)
//   - Bloom filter cache for object-level rate limits
//   - Attribution cache to reduce database queries
//
// Monitoring:
//   - Monkit: defer mon.Task()(&ctx)(&err) on every method
//   - Eventkit: Usage event tracking with project/user-agent
//   - VersionCollector: Tracks uplink client versions (version_collector.go)
//   - NodeSelectionStats: Monitors node selection patterns (node_selection_stats.go)
//
// # Subpackages
//
// expireddeletion/:
//   - Background chore to delete expired objects
//   - Scans metabase for objects past expiration time
//   - Configurable batch size and concurrency
//
// pointerverification/:
//   - Validates piece uploads from storage nodes
//   - Verifies piece hashes and sizes match expectations
//   - Caches node identities for signature verification
//   - Prevents invalid piece data from being stored
//
// bloomrate/:
//   - Space-efficient rate limiting via bloom filters
//   - Lock-free token bucket implementation using atomic operations
//   - Used for per-object upload/download rate limiting
//
// # Object Lock Support
//
// S3-compatible Object Lock features:
//
// Retention Modes:
//   - Governance: Protects object, can be overridden with special permission
//   - Compliance: Absolute protection, cannot be bypassed
//
// Legal Hold:
//   - Indefinite protection flag independent of retention
//   - Can be placed/removed with appropriate permissions
//
// Configuration Levels:
//   - Bucket-level: Object Lock enabled/disabled per bucket
//   - Object-level: Retention and legal hold per object
//
// Enforcement:
//   - Validated in BeginObject when setting retention/legal hold
//   - Checked in BeginDeleteObject and UpdateObjectMetadata
//   - Prevents deletion or modification of locked objects
//
// # Versioning Support
//
// S3-compatible versioning:
//
// Versioning States:
//   - Unversioned: Single version per key (default)
//   - Enabled: Multiple versions per key
//   - Suspended: New uploads create null version
//
// Version Assignment:
//   - Auto-assigned: BeginObject with Version=0 gets next version
//   - Explicit: Client specifies exact version number
//   - StreamVersionID: Public API combining Version + StreamID suffix
//
// Delete Markers:
//   - Soft deletion in versioned buckets
//   - Special object status: DeleteMarkerVersioned, DeleteMarkerUnversioned
//   - ListObjects can optionally include delete markers
//
// # Batch API Optimization
//
// The Batch() and CompressedBatch() methods allow multiple operations in a single RPC:
//
// Supported Batch Operations (batch.go):
//   - BeginObject, CommitObject, GetObject, DownloadObject
//   - BeginSegment, CommitSegment, MakeInlineSegment, DownloadSegment
//   - ListSegments, ListObjects
//   - BeginDeleteObject, FinishCopyObject, FinishMoveObject
//   - GetBucket, CreateBucket, DeleteBucket, ListBuckets
//   - ProjectInfo, GetBucketLocation, GetBucketVersioning, SetBucketVersioning
//   - GetObjectRetention, SetObjectRetention
//   - And more...
//
// Optimizations:
//   - Single network round-trip for multiple operations
//   - Sequential operations to same object skip redundant lookups
//   - Zstd compression reduces bandwidth for large responses
//   - Used heavily by uplink library for efficient uploads/downloads
//
// # Server-Side Copy
//
// Optimization for copying objects without data movement:
//
// How It Works:
//  1. Copies segment metadata in metabase
//  2. New object references same pieces on storage nodes
//  3. No actual data transfer required
//  4. Accounting updated to reflect new object
//
// Benefits:
//   - Near-instant copies regardless of object size
//   - No bandwidth consumption
//   - No storage node involvement
//
// Limitations:
//   - Both objects reference same pieces (deletion affects reference counts)
//   - Source and destination must have compatible encryption
//
// # Attribution Tracking
//
// Partner attribution for analytics and billing:
//
// Attribution Flow (attribution.go):
//  1. Extract user-agent from request
//  2. Parse partner ID from user-agent string
//  3. Ensure bucket has attribution via ensureAttribution()
//  4. Cache attribution to avoid redundant DB updates
//  5. Track usage per partner
//
// Use Cases:
//   - Partner program tracking
//   - Usage analytics per integration
//   - Partner-specific billing
//
// # Testing Support
//
// Test Utilities:
//   - TestingNewAPIKeysEndpoint(): Create minimal endpoint for tests (endpoint.go)
//   - TestSetObjectLockEnabled(): Toggle Object Lock feature (endpoint.go)
//   - TestingSetRSConfig(): Override erasure coding parameters (endpoint.go)
//   - TestingAddTrustedUplink(): Add trusted uplink for tests (endpoint.go)
//
// Test Configuration:
//   - Lower rate limits for faster test execution
//   - Shorter cache expirations
//   - Feature flags for testing new functionality
//
// # Common Workflows
//
// Adding a New RPC Method:
//  1. Define method signature in protocol buffers (storj.io/common/pb)
//  2. Implement method in appropriate endpoint file (object/segment/bucket)
//  3. Add authentication and validation logic
//  4. Coordinate with metabase for persistence
//  5. Add monitoring instrumentation (mon.Task, eventkit)
//  6. Write tests in testplanet
//  7. Update API documentation
//
// Modifying Rate Limits:
//  1. Update Config struct with new limit settings
//  2. Update validation logic to enforce limits
//  3. Test with various load patterns
//  4. Document changes for operators
//
// Adding Feature Flag:
//  1. Add flag to Config struct
//  2. Implement feature with flag check
//  3. Add testing utilities to toggle flag
//  4. Plan rollout and deprecation strategy
//
// # Performance Considerations
//
// Hot Paths:
//   - BeginObject/CommitObject: Upload initiation and finalization
//   - BeginSegment/CommitSegment: Called for every segment
//   - DownloadObject/DownloadSegment: Download path
//
// Optimizations:
//   - LRU caches reduce database queries
//   - Batch API reduces round-trips
//   - Bloom filters provide space-efficient rate limiting
//   - Server-side copy avoids data movement
//   - Attribution cache reduces DB writes
//
// Scaling Considerations:
//   - Stateless design allows horizontal scaling
//   - Rate limiting prevents abuse
//   - Database connection pooling
//   - Caches tune memory vs. database load tradeoff
//
// # Error Handling
//
// Error Classes (endpoint.go):
//   - Error: General metainfo errors
//   - ErrNodeAlreadyExists: Duplicate piece for same node
//   - ErrBucketNotEmpty: Bucket must be empty for deletion
//
// Common gRPC Status Codes:
//   - InvalidArgument: Malformed request
//   - NotFound: Object/bucket not found
//   - PermissionDenied: Authorization failure
//   - ResourceExhausted: Rate limit exceeded
//   - FailedPrecondition: Object Lock constraint violation
//   - Unauthenticated: Missing/invalid API key
//
// # Integration with Satellite
//
// Satellite Peer Integration (satellite/api.go):
//   - Endpoint created in API peer during satellite startup
//   - Registered as gRPC service handler
//   - Dependencies injected from other satellite services
//   - Runs in satellite API process (separate from core services)
//
// Interaction with Other Services:
//   - Metabase: Every operation requires metadata persistence
//   - Orders: Every upload/download requires bandwidth orders
//   - Overlay: BeginSegment requires node selection
//   - Accounting: Usage tracked for billing
//   - Entitlements: Limits enforced on every operation
//
// # Security Considerations
//
// Authentication:
//   - Macaroon-based API keys with cryptographic validation
//   - Time-based expiration and revocation support
//   - Action-based permissions (Read/Write/Delete/List)
//
// Authorization:
//   - Path-based restrictions via macaroon caveats
//   - Bucket-level permissions
//   - Object Lock permission checks
//
// Input Validation:
//   - Request parameter validation (validation.go)
//   - Metadata size limits
//   - Bucket name validation
//   - Path validation
//
// Cryptographic Protections:
//   - Signed stream/segment IDs prevent tampering
//   - HMAC-SHA256 with satellite private key
//   - Piece hash verification
//
// Rate Limiting:
//   - Prevents abuse and DoS attacks
//   - Multiple levels (project, object, user)
//
// # Making Changes
//
// When modifying metainfo code:
//  1. Consider backward compatibility with existing uplinks
//  2. Test with all supported uplink versions
//  3. Maintain gRPC API stability (avoid breaking changes)
//  4. Update protocol buffer definitions if needed
//  5. Add feature flags for gradual rollout
//  6. Test rate limiting and quota enforcement
//  7. Verify Object Lock constraints
//  8. Test with testplanet (full satellite + storage nodes)
//  9. Update documentation and API specs
//  10. Consider performance impact on hot paths
//
// # Key Files Reference
//
//   - endpoint.go: Endpoint struct, initialization, utilities
//   - endpoint_object.go: Object lifecycle operations
//   - endpoint_segment.go: Segment upload/download operations
//   - endpoint_bucket.go: Bucket CRUD operations
//   - endpoint_object_delete.go: Object deletion operations
//   - validation.go: Authentication, authorization, rate limiting
//   - config.go: Configuration struct and erasure coding settings
//   - batch.go: Batch operation handling
//   - signing.go: Cryptographic signing for stream/segment IDs
//   - attribution.go: Partner attribution tracking
//   - success_tracker.go: Node reliability tracking
//   - version_collector.go: Uplink version statistics
//   - node_selection_stats.go: Node selection monitoring
//
// # Related Packages
//
//   - satellite/metabase: Core metadata storage layer
//   - satellite/orders: Bandwidth order management
//   - satellite/overlay: Storage node selection and reputation
//   - satellite/accounting: Usage tracking and billing
//   - satellite/console: User and project management
//   - satellite/buckets: Bucket service layer
//   - satellite/entitlements: Limit enforcement
//   - storj.io/common/pb: Protocol buffer definitions
//   - storj.io/common/macaroon: API key implementation
package metainfo
