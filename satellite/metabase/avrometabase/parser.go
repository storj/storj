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

// record provides error accumulation for parsing Avro record maps.
// Once an error occurs, subsequent operations are skipped.
type record struct {
	recMap map[string]any
	err    error
}

// asRecord creates a new record parser from an Avro record map.
func asRecord(recMap map[string]any) record {
	return record{recMap: recMap}
}

// Err returns the accumulated error, if any.
func (r *record) Err() error {
	return r.err
}

// Int64 extracts an int64 value from the specified field.
// Handles both direct int64 values and Avro union types containing a "long" field.
func (r *record) Int64(field string, dest *int64) {
	if r.err != nil {
		return
	}

	value, found := r.recMap[field]
	if !found {
		return
	}

	switch value := value.(type) {
	case int64:
		*dest = value
	case map[string]any:
		nestedRecord := asRecord(value)
		nestedRecord.Int64("long", dest)
		if nestedRecord.err != nil {
			r.err = nestedRecord.err
		}
	default:
		r.err = errs.New("unable to cast type to int64: %T", value)
	}
}

// Int64AsType extracts an int64 value and converts it to a custom type using the provided function.
func (r *record) Int64AsType(field string, fn func(value int64) error) {
	if r.err != nil {
		return
	}

	var value int64
	r.Int64(field, &value)
	if r.err != nil {
		return
	}

	if err := fn(value); err != nil {
		r.err = errs.New("failed to convert int64: %v", err)
	}
}

// ToInt32 extracts an int64 value and converts it to int32 with overflow checking.
func (r *record) ToInt32(field string, dest *int32) {
	if r.err != nil {
		return
	}

	var value int64
	r.Int64(field, &value)
	if r.err != nil {
		return
	}

	if int64(int32(value)) != value {
		r.err = errs.New("int64 value %d overflows int32", value)
		return
	}

	*dest = int32(value)
}

// Time extracts a time value from the specified field.
// If the field is nil or not present, dest is left unchanged.
func (r *record) Time(field string, dest *time.Time) {
	if r.err != nil {
		return
	}

	var t *time.Time
	r.TimeP(field, &t)
	if r.err != nil {
		return
	}
	if t == nil {
		return
	}
	*dest = *t
}

// TimeP extracts a nullable time value from the specified field.
// Handles both RFC3339 string values and Avro union types containing a "string" field.
func (r *record) TimeP(field string, dest **time.Time) {
	if r.err != nil {
		return
	}

	value, found := r.recMap[field]
	if !found {
		return
	}

	if value == nil {
		*dest = nil
		return
	}

	switch value := value.(type) {
	case string:
		if value == "" {
			return
		}
		t, err := time.Parse(time.RFC3339, value)
		if err != nil {
			r.err = errs.New("failed to parse time: %v", err)
			return
		}
		*dest = &t
	case map[string]any:
		nestedRecord := asRecord(value)
		nestedRecord.TimeP("string", dest)
		if nestedRecord.err != nil {
			r.err = nestedRecord.err
		}
	default:
		r.err = errs.New("unable to cast type to time.Time: %T", value)
	}
}

// Bytes extracts a byte slice from the specified field.
// Handles both direct []byte values and Avro union types containing a "bytes" field.
func (r *record) Bytes(field string, dest *[]byte) {
	if r.err != nil {
		return
	}

	value, found := r.recMap[field]
	if !found {
		return
	}

	if value == nil {
		*dest = nil
		return
	}

	switch value := value.(type) {
	case []byte:
		*dest = value
	case map[string]any:
		nestedRecord := asRecord(value)
		nestedRecord.Bytes("bytes", dest)
		if nestedRecord.err != nil {
			r.err = nestedRecord.err
		}
	default:
		r.err = errs.New("unable to cast type to []byte: %T", value)
	}
}

// String extracts a string value and passes it to the provided callback function.
// Handles string, []byte, and Avro union types containing a "string" field.
func (r *record) String(field string, fn func(value string)) {
	if r.err != nil {
		return
	}

	value, found := r.recMap[field]
	if !found {
		return
	}

	switch value := value.(type) {
	case string:
		fn(value)
	case []byte:
		fn(string(value))
	case map[string]any:
		nestedRecord := asRecord(value)
		nestedRecord.String("string", fn)
		if nestedRecord.err != nil {
			r.err = nestedRecord.err
		}
	default:
		r.err = errs.New("unable to cast field %s to string: %T", field, value)
	}
}

// BytesAsType extracts a byte slice and converts it to a custom type using the provided function.
// The callback function is always invoked, even for nil or empty byte slices, to allow
// proper validation by the conversion function (e.g., uuid.FromBytes, storj.PieceIDFromBytes).
func (r *record) BytesAsType(field string, fn func(value []byte) error) {
	if r.err != nil {
		return
	}

	var b []byte
	r.Bytes(field, &b)
	if r.err != nil {
		return
	}

	if err := fn(b); err != nil {
		r.err = errs.New("failed to convert bytes: %v", err)
	}
}

// SegmentFromRecord parses a segment from an Avro record map.
func SegmentFromRecord(ctx context.Context, recMap map[string]any, aliasCache *metabase.NodeAliasCache) (entry metabase.LoopSegmentEntry, err error) {
	r := asRecord(recMap)
	r.BytesAsType("stream_id", func(value []byte) error {
		entry.StreamID, err = uuid.FromBytes(value)
		return err
	})
	r.Int64AsType("position", func(value int64) error {
		entry.Position = metabase.SegmentPositionFromEncoded(uint64(value))
		return nil
	})
	r.Time("created_at", &entry.CreatedAt)
	r.TimeP("expires_at", &entry.ExpiresAt)
	r.TimeP("repaired_at", &entry.RepairedAt)
	r.BytesAsType("root_piece_id", func(value []byte) error {
		entry.RootPieceID, err = storj.PieceIDFromBytes(value)
		return err
	})
	r.ToInt32("encrypted_size", &entry.EncryptedSize)
	r.Int64("plain_offset", &entry.PlainOffset)
	r.ToInt32("plain_size", &entry.PlainSize)
	r.BytesAsType("remote_alias_pieces", func(value []byte) error {
		return entry.AliasPieces.SetBytes(value)
	})
	r.Int64AsType("redundancy", func(value int64) error {
		return entry.Redundancy.Scan(value)
	})
	r.Int64AsType("placement", func(value int64) error {
		entry.Placement = storj.PlacementConstraint(value)
		return nil
	})
	if err := r.Err(); err != nil {
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
	r := asRecord(recMap)
	r.BytesAsType("project_id", func(value []byte) error {
		entry.ProjectID, err = uuid.FromBytes(value)
		return err
	})
	r.String("bucket_name", func(value string) { entry.BucketName = metabase.BucketName(value) })
	r.String("object_key", func(value string) { entry.ObjectKey = metabase.ObjectKey(value) })
	r.Int64AsType("version", func(value int64) error {
		entry.Version = metabase.Version(value)
		return nil
	})
	r.BytesAsType("stream_id", func(value []byte) error {
		entry.StreamID, err = uuid.FromBytes(value)
		return err
	})
	r.Time("created_at", &entry.CreatedAt)
	r.TimeP("expires_at", &entry.ExpiresAt)
	r.Int64AsType("status", func(value int64) error {
		entry.Status = metabase.ObjectStatus(value)
		return nil
	})
	r.ToInt32("segment_count", &entry.SegmentCount)
	r.Bytes("encrypted_metadata_nonce", &entry.EncryptedMetadataNonce)
	r.Bytes("encrypted_metadata", &entry.EncryptedMetadata)
	r.Bytes("encrypted_metadata_encrypted_key", &entry.EncryptedMetadataEncryptedKey)
	r.Bytes("encrypted_etag", &entry.EncryptedETag)
	r.Int64("total_plain_size", &entry.TotalPlainSize)
	r.Int64("total_encrypted_size", &entry.TotalEncryptedSize)
	r.ToInt32("fixed_segment_size", &entry.FixedSegmentSize)
	r.Int64AsType("encryption", func(value int64) error { return entry.Encryption.Scan(value) })
	r.TimeP("zombie_deletion_deadline", &entry.ZombieDeletionDeadline)
	r.Int64AsType("retention_mode", func(value int64) error {
		var retentionMode metabase.RetentionMode
		err := retentionMode.Scan(value)
		if err != nil {
			return err
		}
		entry.Retention.Mode = retentionMode.Mode
		entry.LegalHold = retentionMode.LegalHold
		return nil
	})
	r.Time("retain_until", &entry.Retention.RetainUntil)
	if err := r.Err(); err != nil {
		return metabase.RawObject{}, Error.Wrap(err)
	}

	return entry, nil
}

// NodeAliasFromRecord parses a NodeAliasEntry from an Avro record map.
func NodeAliasFromRecord(ctx context.Context, recMap map[string]any) (entry metabase.NodeAliasEntry, err error) {
	r := asRecord(recMap)
	r.BytesAsType("node_id", func(value []byte) error {
		entry.ID, err = storj.NodeIDFromBytes(value)
		return err
	})
	r.Int64AsType("node_alias", func(value int64) error {
		entry.Alias = metabase.NodeAlias(value)
		return nil
	})
	if err := r.Err(); err != nil {
		return metabase.NodeAliasEntry{}, Error.Wrap(err)
	}

	return entry, nil
}
