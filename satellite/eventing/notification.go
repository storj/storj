// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
)

// S3ObjectEvent represents various event names triggered by S3 object operations.
const (
	S3ObjectCreatedPut                 = "ObjectCreated:Put"
	S3ObjectCreatedCopy                = "ObjectCreated:Copy"
	S3ObjectRemovedDelete              = "ObjectRemoved:Delete"
	S3ObjectRemovedDeleteMarkerCreated = "ObjectRemoved:DeleteMarkerCreated"
)

// Event contains one or more event records.
type Event struct {
	Records []EventRecord `json:"Records,omitempty"`
}

// EventRecord represents a change of a database record. Modeled to be compatible with similar events from AWS.
type EventRecord struct {
	EventVersion string `json:"eventVersion,omitempty"`
	EventSource  string `json:"eventSource,omitempty"`
	AwsRegion    string `json:"awsRegion,omitempty"`
	EventTime    string `json:"eventTime,omitempty"`
	EventName    string `json:"eventName,omitempty"`
	UserIdentity struct {
		PrincipalId string `json:"principalId,omitempty"`
	} `json:"userIdentity,omitempty"`
	RequestParameters struct {
		SourceIPAddress string `json:"sourceIPAddress,omitempty"`
	} `json:"requestParameters,omitempty"`
	ResponseElements struct {
		XAmzRequestId string `json:"x-amz-request-id,omitempty"`
		XAmzId2       string `json:"x-amz-id-2,omitempty"`
	} `json:"responseElements"`
	S3 struct {
		S3SchemaVersion string `json:"s3SchemaVersion,omitempty"`
		ConfigurationId string `json:"configurationId,omitempty"`
		Bucket          struct {
			Name          string `json:"name,omitempty"`
			OwnerIdentity struct {
				PrincipalId string `json:"principalId,omitempty"`
			} `json:"ownerIdentity,omitempty"`
			Arn string `json:"arn,omitempty"`
		} `json:"bucket"`
		Object struct {
			Key       string `json:"key,omitempty"`
			Size      int64  `json:"size,omitempty"`
			ETag      string `json:"eTag,omitempty"`
			VersionId string `json:"versionId,omitempty"`
			Sequencer string `json:"sequencer,omitempty"`
		} `json:"object,omitempty"`
	} `json:"s3,omitempty"`
}

// ISO8601 is the time format used in S3 event notifications.
const ISO8601 = "2006-01-02T15:04:05.000Z"

// ConvertModsToEvent converts a DataChangeRecord into an Event containing EventRecords.
func ConvertModsToEvent(dataRecord changestream.DataChangeRecord) (event Event, err error) {
	for _, mod := range dataRecord.Mods {
		record := EventRecord{}

		record.EventVersion = "2.1"
		record.EventSource = "storj:s3"
		record.EventTime = dataRecord.CommitTimestamp.UTC().Format(ISO8601)
		record.S3.S3SchemaVersion = "1.0"
		record.S3.ConfigurationId = "ObjectEvents"

		eventName := determineEventName(dataRecord.TransactionTag, dataRecord.ModType)
		if eventName == "" {
			continue
		}
		record.EventName = eventName

		var keys, oldValues, newValues map[string]interface{}

		keys, err = parseNullJSONMap(mod.Keys, "keys")
		if err != nil {
			return Event{}, err
		}

		oldValues, err = parseNullJSONMap(mod.OldValues, "old values")
		if err != nil {
			return Event{}, err
		}

		newValues, err = parseNullJSONMap(mod.NewValues, "new values")
		if err != nil {
			return Event{}, err
		}

		var version int64
		var ok bool
		if version, ok = extractInt64("version", keys); !ok {
			continue
		}

		if bucketName, ok := extractString("bucket_name", keys); ok {
			record.S3.Bucket.Name = bucketName
			record.S3.Bucket.Arn = fmt.Sprintf("arn:storj:s3:::%s", bucketName)
		}

		if projectID, ok := extractString("project_id", keys); ok {
			projectIDBytes, err := base64.StdEncoding.DecodeString(projectID)
			if err != nil {
				return Event{}, errs.New("invalid base64 project_id: %w", err)
			}
			projectID, err := uuid.FromBytes(projectIDBytes)
			if err != nil {
				return Event{}, errs.New("invalid project_id uuid: %w", err)
			}
			record.S3.Bucket.OwnerIdentity.PrincipalId = projectID.String()
		}

		if objectKey, ok := extractString("object_key", keys); ok {
			objectKeyBytes, err := base64.StdEncoding.DecodeString(objectKey)
			if err != nil {
				return Event{}, errs.New("invalid base64 object_key: %w", err)
			}
			record.S3.Object.Key = string(objectKeyBytes)
		}

		if totalPlainSize, ok := extractFirstInt64("total_plain_size", newValues, oldValues); ok {
			record.S3.Object.Size = totalPlainSize
		}

		if streamID, ok := extractFirstString("stream_id", newValues, oldValues); ok {
			streamIDBytes, err := base64.StdEncoding.DecodeString(streamID)
			if err != nil {
				return Event{}, errs.New("invalid base64 stream_id: %w", err)
			}
			streamID, err := uuid.FromBytes(streamIDBytes)
			if err != nil {
				return Event{}, errs.New("invalid stream_id uuid: %w", err)
			}
			streamVersionID := metabase.NewStreamVersionID(metabase.Version(version), streamID)
			record.S3.Object.VersionId = hex.EncodeToString(streamVersionID.Bytes())
		}

		commitNanos := dataRecord.CommitTimestamp.UnixNano()
		record.S3.Object.Sequencer = fmt.Sprintf("%016X", commitNanos)

		event.Records = append(event.Records, record)
	}

	return event, nil
}

// parseNullJSONMap converts a spanner.NullJSON into a map[string]interface{}.
// It supports values represented as a JSON string or already decoded map.
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
		// Best-effort: marshal then unmarshal to map
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

func determineEventName(transactionTag, modType string) string {
	switch transactionTag {
	case "commit-inline-object":
		if modType == "INSERT" {
			return S3ObjectCreatedPut
		}
	case "commit-object":
		switch modType {
		case "INSERT", "UPDATE":
			return S3ObjectCreatedPut
		}
	case "delete-all-bucket-objects", "delete-object-exact-version", "delete-object-exact-version-using-object-lock", "delete-object-last-committed-plain":
		if modType == "DELETE" {
			return S3ObjectRemovedDelete
		}
	case "delete-object-last-committed-suspended", "delete-object-last-committed-versioned":
		if modType == "INSERT" {
			return S3ObjectRemovedDeleteMarkerCreated
		}
	case "finish-copy-object":
		if modType == "UPDATE" {
			return S3ObjectCreatedCopy
		}
	case "finish-move-object":
		if modType == "INSERT" {
			return S3ObjectCreatedCopy
		}
	}
	return ""
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

// JSONSize returns the message length.
func (e *Event) JSONSize() (int64, error) {
	eventJSON, err := json.Marshal(e)
	if err != nil {
		return 0, err
	}
	return int64(len(eventJSON)), nil
}
