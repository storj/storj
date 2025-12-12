// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package avrometabase

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// Error is the base error class for avrometabase package errors.
var Error = errs.Class("avrometabase")

// int64Field extracts an int64 value from the specified field.
// Handles both direct int64 values and Avro union types containing a "long" field.
func int64Field(recMap map[string]any, field string, dest *int64) error {
	value, found := recMap[field]
	if !found {
		return nil
	}

	switch value := value.(type) {
	case int64:
		*dest = value
		return nil
	case map[string]any:
		return int64Field(value, "long", dest)
	default:
		return errs.New("unable to cast type to int64: %T", value)
	}
}

// int64AsType extracts an int64 value and converts it to a custom type using the provided function.
func int64AsType(recMap map[string]any, field string, fn func(value int64) error) error {
	var value int64
	if err := int64Field(recMap, field, &value); err != nil {
		return err
	}

	if err := fn(value); err != nil {
		return errs.New("failed to convert int64: %v", err)
	}
	return nil
}

// toInt32 extracts an int64 value and converts it to int32 with overflow checking.
func toInt32(recMap map[string]any, field string, dest *int32) error {
	var value int64
	if err := int64Field(recMap, field, &value); err != nil {
		return err
	}

	if int64(int32(value)) != value {
		return errs.New("int64 value %d overflows int32", value)
	}

	*dest = int32(value)
	return nil
}

// timeField extracts a time value from the specified field.
// If the field is nil or not present, dest is left unchanged.
func timeField(recMap map[string]any, field string, dest *time.Time) error {
	var t *time.Time
	if err := timePField(recMap, field, &t); err != nil {
		return err
	}
	if t == nil {
		return nil
	}
	*dest = *t
	return nil
}

// timePField extracts a nullable time value from the specified field.
// Handles both RFC3339 string values and Avro union types containing a "string" field.
func timePField(recMap map[string]any, field string, dest **time.Time) error {
	value, found := recMap[field]
	if !found {
		return nil
	}

	if value == nil {
		*dest = nil
		return nil
	}

	switch value := value.(type) {
	case string:
		if value == "" {
			return nil
		}
		t, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return errs.New("failed to parse time: %v", err)
		}
		*dest = &t
		return nil
	case map[string]any:
		return timePField(value, "string", dest)
	default:
		return errs.New("unable to cast type to time.Time: %T", value)
	}
}

// bytesField extracts a byte slice from the specified field.
// Handles both direct []byte values and Avro union types containing a "bytes" field.
func bytesField(recMap map[string]any, field string, dest *[]byte) error {
	value, found := recMap[field]
	if !found {
		return nil
	}

	if value == nil {
		*dest = nil
		return nil
	}

	switch value := value.(type) {
	case []byte:
		*dest = value
		return nil
	case map[string]any:
		return bytesField(value, "bytes", dest)
	default:
		return errs.New("unable to cast type to []byte: %T", value)
	}
}

// stringField extracts a string value and passes it to the provided callback function.
// Handles string, []byte, and Avro union types containing a "string" field.
func stringField(recMap map[string]any, field string, fn func(value string)) error {
	value, found := recMap[field]
	if !found {
		return nil
	}

	switch value := value.(type) {
	case string:
		fn(value)
		return nil
	case []byte:
		fn(string(value))
		return nil
	case map[string]any:
		return stringField(value, "string", fn)
	default:
		return errs.New("unable to cast field %s to string: %T", field, value)
	}
}

// bytesAsType extracts a byte slice and converts it to a custom type using the provided function.
// The callback function is always invoked, even for nil or empty byte slices, to allow
// proper validation by the conversion function (e.g., uuid.FromBytes, storj.PieceIDFromBytes).
func bytesAsType(recMap map[string]any, field string, fn func(value []byte) error) error {
	var b []byte
	if err := bytesField(recMap, field, &b); err != nil {
		return err
	}

	if err := fn(b); err != nil {
		return errs.New("failed to convert bytes: %v", err)
	}
	return nil
}

// SegmentFromRecord parses a segment from an Avro record map.
func SegmentFromRecord(ctx context.Context, recMap map[string]any, aliasCache *metabase.NodeAliasCache) (entry metabase.LoopSegmentEntry, err error) {
	err = errs.Combine(
		bytesAsType(recMap, "stream_id", func(value []byte) error {
			entry.StreamID, err = uuid.FromBytes(value)
			return err
		}),
		int64AsType(recMap, "position", func(value int64) error {
			entry.Position = metabase.SegmentPositionFromEncoded(uint64(value))
			return nil
		}),
		timeField(recMap, "created_at", &entry.CreatedAt),
		timePField(recMap, "expires_at", &entry.ExpiresAt),
		timePField(recMap, "repaired_at", &entry.RepairedAt),
		bytesAsType(recMap, "root_piece_id", func(value []byte) error {
			entry.RootPieceID, err = storj.PieceIDFromBytes(value)
			return err
		}),
		toInt32(recMap, "encrypted_size", &entry.EncryptedSize),
		int64Field(recMap, "plain_offset", &entry.PlainOffset),
		toInt32(recMap, "plain_size", &entry.PlainSize),
		bytesAsType(recMap, "remote_alias_pieces", func(value []byte) error {
			return entry.AliasPieces.SetBytes(value)
		}),
		int64AsType(recMap, "redundancy", func(value int64) error {
			return entry.Redundancy.Scan(value)
		}),
		int64AsType(recMap, "placement", func(value int64) error {
			entry.Placement = storj.PlacementConstraint(value)
			return nil
		}),
	)
	if err != nil {
		return metabase.LoopSegmentEntry{}, Error.Wrap(err)
	}

	// TODO may think about memory optimization here
	entry.Pieces, err = aliasCache.ConvertAliasesToPieces(ctx, entry.AliasPieces)
	if err != nil {
		return metabase.LoopSegmentEntry{}, Error.Wrap(err)
	}

	entry.Source = "avro"
	return entry, nil
}

// ObjectFromRecord parses a RawObject from an Avro record map.
func ObjectFromRecord(ctx context.Context, recMap map[string]any) (entry metabase.RawObject, err error) {
	err = errs.Combine(
		bytesAsType(recMap, "project_id", func(value []byte) error {
			entry.ProjectID, err = uuid.FromBytes(value)
			return err
		}),
		stringField(recMap, "bucket_name", func(value string) { entry.BucketName = metabase.BucketName(value) }),
		stringField(recMap, "object_key", func(value string) { entry.ObjectKey = metabase.ObjectKey(value) }),
		int64AsType(recMap, "version", func(value int64) error {
			entry.Version = metabase.Version(value)
			return nil
		}),
		bytesAsType(recMap, "stream_id", func(value []byte) error {
			entry.StreamID, err = uuid.FromBytes(value)
			return err
		}),
		timeField(recMap, "created_at", &entry.CreatedAt),
		timePField(recMap, "expires_at", &entry.ExpiresAt),
		int64AsType(recMap, "status", func(value int64) error {
			entry.Status = metabase.ObjectStatus(value)
			return nil
		}),
		toInt32(recMap, "segment_count", &entry.SegmentCount),
		bytesField(recMap, "encrypted_metadata_nonce", &entry.EncryptedMetadataNonce),
		bytesField(recMap, "encrypted_metadata", &entry.EncryptedMetadata),
		bytesField(recMap, "encrypted_metadata_encrypted_key", &entry.EncryptedMetadataEncryptedKey),
		bytesField(recMap, "encrypted_etag", &entry.EncryptedETag),
		int64Field(recMap, "total_plain_size", &entry.TotalPlainSize),
		int64Field(recMap, "total_encrypted_size", &entry.TotalEncryptedSize),
		toInt32(recMap, "fixed_segment_size", &entry.FixedSegmentSize),
		int64AsType(recMap, "encryption", func(value int64) error { return entry.Encryption.Scan(value) }),
		timePField(recMap, "zombie_deletion_deadline", &entry.ZombieDeletionDeadline),
		int64AsType(recMap, "retention_mode", func(value int64) error {
			var retentionMode metabase.RetentionMode
			err := retentionMode.Scan(value)
			if err != nil {
				return err
			}
			entry.Retention.Mode = retentionMode.Mode
			entry.LegalHold = retentionMode.LegalHold
			return nil
		}),
		timeField(recMap, "retain_until", &entry.Retention.RetainUntil),
	)
	if err != nil {
		return metabase.RawObject{}, Error.Wrap(err)
	}

	return entry, nil
}

// NodeAliasFromRecord parses a NodeAliasEntry from an Avro record map.
func NodeAliasFromRecord(ctx context.Context, recMap map[string]any) (entry metabase.NodeAliasEntry, err error) {
	err = errs.Combine(
		bytesAsType(recMap, "node_id", func(value []byte) error {
			entry.ID, err = storj.NodeIDFromBytes(value)
			return err
		}),
		int64AsType(recMap, "node_alias", func(value int64) error {
			entry.Alias = metabase.NodeAlias(value)
			return nil
		}),
	)
	if err != nil {
		return metabase.NodeAliasEntry{}, Error.Wrap(err)
	}

	return entry, nil
}
