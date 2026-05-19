// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/eventkit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
)

// SpannerEventSource implements EventSource by wrapping changestream.Processor.
// It decodes each DataChangeRecord into a ChangeEvent before calling fn.
type SpannerEventSource struct {
	log      *zap.Logger
	adapter  changestream.Adapter
	feedName string
}

// NewSpannerEventSource creates a SpannerEventSource.
func NewSpannerEventSource(log *zap.Logger, adapter changestream.Adapter, feedName string) *SpannerEventSource {
	return &SpannerEventSource{
		log:      log,
		adapter:  adapter,
		feedName: feedName,
	}
}

// Listen starts the Spanner change stream processing loop and calls fn for each
// decoded ChangeEvent. Blocks until ctx is cancelled or a permanent error occurs.
func (s *SpannerEventSource) Listen(ctx context.Context, fn func(ChangeEvent) (PendingResult, error)) error {
	return changestream.Processor(ctx, s.log, s.adapter, s.feedName, time.Now(),
		func(record changestream.DataChangeRecord) (PendingResult, error) {
			return s.processRecord(ctx, record, fn)
		},
	)
}

func (s *SpannerEventSource) processRecord(ctx context.Context, record changestream.DataChangeRecord, fn func(ChangeEvent) (PendingResult, error)) (_ PendingResult, err error) {
	defer mon.Task()(&ctx)(&err)

	events, err := ConvertModsToEvents(record)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		ek.Event("change_record_no_events",
			eventkit.String("table", record.TableName),
			eventkit.String("mod_type", record.ModType),
			eventkit.String("transaction_tag", record.TransactionTag),
			eventkit.Int64("mods_count", int64(len(record.Mods))))
		return ImmediateResult(record.CommitTimestamp), nil
	}

	var results []PendingResult
	for _, event := range events {
		result, err := fn(event)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	if len(results) == 1 {
		return results[0], nil
	}
	return NewCombinedPendingResult(results), nil
}

// ConvertModsToEvents decodes a DataChangeRecord into zero or more ChangeEvents,
// one per mod that maps to a recognised S3 event type.
func ConvertModsToEvents(dataRecord changestream.DataChangeRecord) ([]ChangeEvent, error) {
	var events []ChangeEvent

	for _, mod := range dataRecord.Mods {
		eventName := determineEventName(dataRecord.TransactionTag, dataRecord.ModType)
		if eventName == "" {
			continue
		}

		keys, err := parseNullJSONMap(mod.Keys, "keys")
		if err != nil {
			return nil, err
		}
		oldValues, err := parseNullJSONMap(mod.OldValues, "old values")
		if err != nil {
			return nil, err
		}
		newValues, err := parseNullJSONMap(mod.NewValues, "new values")
		if err != nil {
			return nil, err
		}

		version, ok := extractInt64("version", keys)
		if !ok {
			continue
		}

		var projectID uuid.UUID
		if projectIDStr, ok := extractString("project_id", keys); ok {
			projectIDBytes, err := base64.StdEncoding.DecodeString(projectIDStr)
			if err != nil {
				return nil, errs.New("invalid base64 project_id: %w", err)
			}
			projectID, err = uuid.FromBytes(projectIDBytes)
			if err != nil {
				return nil, errs.New("invalid project_id uuid: %w", err)
			}
		}

		var bucketName metabase.BucketName
		if bn, ok := extractString("bucket_name", keys); ok {
			bucketName = metabase.BucketName(bn)
		}

		var objectKey metabase.ObjectKey
		if okStr, ok := extractString("object_key", keys); ok {
			objectKeyBytes, err := base64.StdEncoding.DecodeString(okStr)
			if err != nil {
				return nil, errs.New("invalid base64 object_key: %w", err)
			}
			objectKey = metabase.ObjectKey(objectKeyBytes)
		}

		var totalPlainSize int64
		if tps, ok := extractFirstInt64("total_plain_size", newValues, oldValues); ok {
			totalPlainSize = tps
		}

		var streamID uuid.UUID
		if streamIDStr, ok := extractFirstString("stream_id", newValues, oldValues); ok {
			streamIDBytes, err := base64.StdEncoding.DecodeString(streamIDStr)
			if err != nil {
				return nil, errs.New("invalid base64 stream_id: %w", err)
			}
			streamID, err = uuid.FromBytes(streamIDBytes)
			if err != nil {
				return nil, errs.New("invalid stream_id uuid: %w", err)
			}
		}

		events = append(events, ChangeEvent{
			EventName: eventName,
			ObjectStream: metabase.ObjectStream{
				ProjectID:  projectID,
				BucketName: bucketName,
				ObjectKey:  objectKey,
				Version:    metabase.Version(version),
				StreamID:   streamID,
			},
			TotalPlainSize:  totalPlainSize,
			CommitTimestamp: dataRecord.CommitTimestamp,
		})
	}

	return events, nil
}

func determineEventName(transactionTag, modType string) string {
	switch transactionTag {
	case "commit-inline-object":
		if modType == "INSERT" {
			return EventNameObjectCreatedPut
		}
	case "commit-object":
		switch modType {
		case "INSERT", "UPDATE":
			return EventNameObjectCreatedPut
		}
	case "delete-all-bucket-objects", "delete-object-exact-version", "delete-object-exact-version-using-object-lock", "delete-object-last-committed-plain":
		if modType == "DELETE" {
			return EventNameObjectRemovedDelete
		}
	case "delete-object-last-committed-suspended", "delete-object-last-committed-versioned":
		if modType == "INSERT" {
			return EventNameObjectRemovedDeleteMarkerCreated
		}
	case "finish-copy-object":
		if modType == "UPDATE" {
			return EventNameObjectCreatedCopy
		}
	case "finish-move-object":
		switch modType {
		case "INSERT":
			return EventNameObjectCreatedCopy
		case "DELETE":
			return EventNameObjectRemovedDelete
		}
	}
	return ""
}

// parseNullJSONMap converts a spanner.NullJSON into a map[string]interface{}.
// Returns (nil, nil) when the value is invalid or empty.
func parseNullJSONMap(nj spanner.NullJSON, what string) (map[string]interface{}, error) {
	if !nj.Valid || nj.Value == nil {
		return nil, nil
	}
	var out map[string]interface{}
	switch v := nj.Value.(type) {
	case string:
		if v == "" {
			return nil, nil
		}
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, errs.New("failed to unmarshal %s: %w", what, err)
		}
		return out, nil
	case map[string]interface{}:
		return v, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, errs.New("failed to marshal %s: %w", what, err)
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return nil, errs.New("failed to unmarshal %s: %w", what, err)
		}
		return out, nil
	}
}

func extractString(key string, values map[string]interface{}) (string, bool) {
	if values == nil {
		return "", false
	}
	if val, ok := values[key]; ok {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}

// extractFirstString calls extractString on the first values map, if not found continue to the next ones.
func extractFirstString(key string, values ...map[string]interface{}) (string, bool) {
	for _, v := range values {
		if iv, ok := extractString(key, v); ok {
			return iv, true
		}
	}
	return "", false
}

func extractInt64(key string, values map[string]interface{}) (int64, bool) {
	if values == nil {
		return 0, false
	}
	if val, ok := values[key]; ok {
		switch v := val.(type) {
		case int64:
			return v, true
		case float64:
			return int64(v), true
		case string:
			iv, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return 0, false
			}
			return iv, true
		case json.Number:
			if i, err := v.Int64(); err == nil {
				return i, true
			}
		}
	}
	return 0, false
}

// extractFirstInt64 calls extractInt64 on the first values map, if not found continue to the next ones.
func extractFirstInt64(key string, values ...map[string]interface{}) (int64, bool) {
	for _, v := range values {
		if iv, ok := extractInt64(key, v); ok {
			return iv, true
		}
	}
	return 0, false
}
