// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package avrometabase provides utilities for parsing segment metadata from Avro files,
// enabling bulk import and processing of segment data from external sources.
//
// # Overview
//
// This package enables reading metabase segment data stored in Avro format, which is useful for:
//   - Processing Spanner change stream exports
//   - Bulk data migration scenarios
//   - Historical data analysis
//   - Backup/restore operations
//   - Offline processing of segment metadata
//
// Avro is a binary data serialization format that provides:
//   - Compact binary encoding
//   - Self-describing schema
//   - Language-independent serialization
//   - Efficient for large-scale data processing
//
// # Architecture
//
// The package provides two main components:
//
// 1. Iterators (iterator.go): Read Avro files from various sources
// 2. Parser (parser.go): Convert Avro records to metabase types
//
// # Iterators
//
// ReaderIterator interface (iterator.go:20):
//   - Next(ctx): Returns io.ReadCloser for next Avro file
//   - Returns nil when no more files available
//   - Thread-safe with mutex protection
//
// FileIterator (iterator.go:25):
//   - Reads Avro files from local filesystem
//   - Uses glob patterns for file matching
//   - Example: NewFileIterator("/data/segments-*.avro")
//   - Lazily initializes file list on first Next() call
//
// Usage:
//
//	iter := avrometabase.NewFileIterator("/path/to/segments-*.avro")
//	for {
//	    reader, err := iter.Next(ctx)
//	    if reader == nil { break } // No more files
//	    // Process Avro file
//	    reader.Close()
//	}
//
// GCSIterator (iterator.go:70):
//   - Reads Avro files from Google Cloud Storage
//   - Uses GCS object name patterns for matching
//   - Requires authenticated GCS client
//   - Example: NewGCSIterator(client, "my-bucket", "segments/")
//
// Usage:
//
//	client, _ := storage.NewClient(ctx)
//	iter := avrometabase.NewGCSIterator(client, "bucket", "prefix")
//	for {
//	    reader, err := iter.Next(ctx)
//	    if reader == nil { break }
//	    // Process Avro file from GCS
//	    reader.Close()
//	}
//
// # Parser
//
// SegmentFromRecord (parser.go:18):
//   - Converts Avro record map to metabase.LoopSegmentEntry
//   - Handles Avro's type encoding (unions, optional fields)
//   - Resolves node aliases to NodeIDs via NodeAliasCache
//
// The function extracts all segment fields:
//   - stream_id: UUID identifying the object
//   - position: Encoded segment position (part + index)
//   - created_at, expires_at, repaired_at: Timestamps
//   - root_piece_id: Root piece identifier for erasure coding
//   - encrypted_size, plain_offset, plain_size: Size information
//   - remote_alias_pieces: Compact piece locations (node aliases)
//   - redundancy: Erasure coding scheme
//   - placement: Geographic placement constraint
//
// Usage:
//
//	// Read Avro record (using goavro or similar)
//	record, _ := ocfReader.Read()
//	recMap := record.(map[string]interface{})
//
//	// Parse to segment
//	segment, err := avrometabase.SegmentFromRecord(ctx, recMap, aliasCache)
//	if err != nil {
//	    // Handle parse error
//	}
//
//	// segment is now a metabase.LoopSegmentEntry ready for processing
//
// # Avro Type Handling
//
// Avro uses specific type encoding that requires special handling:
//
// Nullable Fields (Union Types):
//   - Avro represents nullable as union: ["null", "type"]
//   - Encoded as map[string]any with single key
//   - ToInt64(field) handles: int64, map["long"]int64, map["null"]nil
//   - ToTimeP(field) handles optional timestamps
//
// Timestamps:
//   - Avro logical type: timestamp-micros (int64 microseconds)
//   - ToTime(field) converts to time.Time (parser.go:131)
//   - ToTimeP(field) converts optional timestamp (parser.go:142)
//
// Binary Data:
//   - Avro bytes type maps to []byte
//   - ToBytes(field) extracts byte arrays (parser.go:166)
//   - BytesToType(field, fn) converts bytes to typed values (parser.go:182)
//
// Example Avro type conversions (parser.go:115-189):
//
//	// Required int64
//	positionEncoded, err := ToInt64(recMap["position"])
//
//	// Optional timestamp
//	expiresAt, err := ToTimeP(recMap["expires_at"]) // *time.Time
//
//	// Binary UUID
//	streamID, err := BytesToType(recMap["stream_id"], uuid.FromBytes)
//
//	// Binary data
//	aliasPiecesBytes, err := ToBytes(recMap["remote_alias_pieces"])
//
// # Node Alias Resolution
//
// Avro segment records contain remote_alias_pieces (compact node aliases),
// but LoopSegmentEntry needs full Pieces (with NodeIDs).
//
// SegmentFromRecord uses NodeAliasCache to resolve:
//
//	aliasPieces.SetBytes(aliasPiecesBytes) // Parse compact format
//	pieces, err := aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
//
// The cache queries the database for NodeID mappings, caching results
// for efficiency when processing many segments.
//
// Important: Pass a shared NodeAliasCache instance when processing
// multiple segments to benefit from caching.
//
// # Integration with Ranged Loop
//
// This package integrates with metabase/rangedloop for parallel processing:
//
//	// provider_avro.go uses avrometabase to create segment providers
//	type AvroRangeSplitter struct {
//	    iterator ReaderIterator
//	}
//
//	func (s *AvroRangeSplitter) CreateRanges(n int) []SegmentProvider {
//	    // Distribute Avro files across ranges
//	    // Each provider reads subset of files
//	}
//
// This enables parallel processing of Avro segment exports using the
// same observer pattern as database-backed segment iteration.
//
// # Use Cases
//
// Spanner Change Streams:
//   - Spanner exports change stream to Avro files in GCS
//   - Use GCSIterator to read exported files
//   - Process segment changes for analytics or replication
//
// Data Migration:
//   - Export segments from one satellite to Avro
//   - Import to another satellite or database
//   - Useful for datacenter migrations
//
// Historical Analysis:
//   - Archive segment snapshots as Avro files
//   - Analyze trends over time
//   - Reproduce past states for debugging
//
// Offline Processing:
//   - Download Avro exports for local processing
//   - Run analysis without impacting production database
//   - Generate reports or statistics
//
// # Performance Considerations
//
// Avro Efficiency:
//   - Binary format is compact and fast to parse
//   - Self-describing schema reduces parsing overhead
//   - Suitable for processing millions of segments
//
// Memory Usage:
//   - Iterators read one file at a time
//   - Parser allocates LoopSegmentEntry per segment
//   - NodeAliasCache accumulates mappings (grows with unique nodes)
//
// Optimization Tips:
//   - Reuse NodeAliasCache across segments
//   - Process files in parallel with multiple goroutines
//   - Consider batching segments before processing
//   - Stream large files rather than loading entirely
//
// # Error Handling
//
// Parse errors are wrapped with context:
//
//	errs.Wrap(err) // Maintains error chain
//
// Common errors:
//   - Invalid Avro encoding (wrong type)
//   - Missing required fields
//   - Node alias not found in cache
//   - Invalid timestamp format
//   - Corrupt binary data
//
// Always check errors from SegmentFromRecord and handle appropriately.
//
// # Avro Schema Assumptions
//
// The parser expects Avro records with specific field names and types
// matching the metabase segments table schema:
//
// Required fields:
//   - stream_id: bytes (UUID)
//   - position: long (encoded position)
//   - created_at: long (timestamp-micros)
//   - root_piece_id: bytes (PieceID)
//   - encrypted_size: long
//   - plain_offset: long
//   - plain_size: long
//   - remote_alias_pieces: bytes (compressed piece list)
//   - redundancy: long (encoded redundancy scheme)
//   - placement: long (placement constraint)
//
// Optional fields (nullable unions):
//   - expires_at: ["null", "long"] (timestamp-micros)
//   - repaired_at: ["null", "long"] (timestamp-micros)
//
// Schema version must match the metabase version being used.
//
// # Testing
//
// When testing Avro parsing:
//  1. Generate test Avro files with goavro or similar
//  2. Include edge cases: null values, empty pieces, etc.
//  3. Test both FileIterator and GCSIterator paths
//  4. Verify node alias resolution works correctly
//  5. Check error handling for malformed data
//
// # Making Changes
//
// When modifying Avro parsing:
//   - Ensure changes match metabase schema version
//   - Update type conversions if Avro schema changes
//   - Test with real Spanner exports if possible
//   - Consider backward compatibility with old exports
//   - Document any schema version requirements
//
// When adding new iterators:
//   - Implement ReaderIterator interface
//   - Ensure thread-safety with mutex
//   - Return nil reader when exhausted
//   - Close resources properly
//
// # Related Packages
//
//   - metabase: Core segment types (LoopSegmentEntry)
//   - metabase/rangedloop: Parallel segment processing
//   - metabase/changestream: Spanner change data capture
//   - goavro: Avro encoding/decoding library
//
// # Common Issues
//
// Node alias not found:
//   - Ensure NodeAliasCache has access to metabase DB
//   - Check that aliases exist in node_aliases table
//   - Verify Avro data matches database version
//
// Parse errors:
//   - Verify Avro schema matches metabase version
//   - Check for corrupt Avro files
//   - Ensure all required fields present
//
// Memory growth:
//   - NodeAliasCache grows with unique nodes
//   - Consider periodic cache clearing for long runs
//   - Process files in smaller batches if needed
//
// GCS authentication:
//   - Ensure GCS client is properly authenticated
//   - Check IAM permissions on bucket
//   - Verify bucket and prefix exist
package avrometabase
