// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
)

// Event contains one or more event records.
type Event struct {
	Bucket  metabase.BucketLocation `json:"-"`
	Records []EventRecord           `json:"Records,omitempty"`
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

		var newValues, oldValues, keys map[string]interface{}

		newValues, err = parseNullJSONMap(mod.NewValues, "new values")
		if err != nil {
			return Event{}, err
		}

		oldValues, err = parseNullJSONMap(mod.OldValues, "old values")
		if err != nil {
			return Event{}, err
		}

		keys, err = parseNullJSONMap(mod.Keys, "keys")
		if err != nil {
			return Event{}, err
		}

		eventName := determineEventName(dataRecord.ModType, newValues, oldValues)
		if eventName == "" {
			continue
		}
		record.EventName = eventName

		if bucketName, ok := extractString(keys, "bucket_name"); ok {
			record.S3.Bucket.Name = bucketName
			record.S3.Bucket.Arn = fmt.Sprintf("arn:storj:s3:::%s", bucketName)
			// TODO: what if mods span multiple buckets?
			event.Bucket.BucketName = metabase.BucketName(bucketName)
		}

		if projectID, ok := extractString(keys, "project_id"); ok {
			projectIDBytes, err := base64.StdEncoding.DecodeString(projectID)
			if err != nil {
				return Event{}, errs.New("invalid base64 project_id: %w", err)
			}
			// TODO: what if mods span multiple projects?
			event.Bucket.ProjectID, err = uuid.FromBytes(projectIDBytes)
			if err != nil {
				return Event{}, errs.New("invalid project_id uuid: %w", err)
			}
			// TODO: look up the public project ID and set it as the bucket owner
			// record.S3.Bucket.OwnerIdentity.PrincipalId = publicProjectID
		}

		if objectKey, ok := extractString(keys, "object_key"); ok {
			objectKeyBytes, err := base64.StdEncoding.DecodeString(objectKey)
			if err != nil {
				return Event{}, errs.New("invalid base64 object_key: %w", err)
			}
			record.S3.Object.Key = string(objectKeyBytes)
		}

		if totalPlainSize, ok := extractInt64(newValues, "total_plain_size"); ok {
			record.S3.Object.Size = totalPlainSize
		}

		if version, ok := extractInt64(keys, "version"); ok {
			record.S3.Object.VersionId = strconv.FormatInt(version, 10)
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

func determineEventName(modType string, newValues, oldValues map[string]interface{}) string {
	switch modType {
	case "INSERT":
		if newStatus, ok := extractInt64(newValues, "status"); ok {
			switch metabase.ObjectStatus(newStatus) {
			case metabase.CommittedUnversioned, metabase.CommittedVersioned:
				return "ObjectCreated:Put"
			case metabase.DeleteMarkerVersioned, metabase.DeleteMarkerUnversioned:
				return "ObjectRemoved:DeleteMarkerCreated"
			}
		}
	case "UPDATE":
		if newStatus, ok := extractInt64(newValues, "status"); ok {
			if oldStatus, ok := extractInt64(oldValues, "status"); ok && metabase.ObjectStatus(oldStatus) == metabase.Pending {
				switch metabase.ObjectStatus(newStatus) {
				case metabase.CommittedUnversioned, metabase.CommittedVersioned:
					return "ObjectCreated:Put"
				}
			}
		}
	case "DELETE":
		if oldStatus, ok := extractInt64(oldValues, "status"); ok {
			switch metabase.ObjectStatus(oldStatus) {
			case metabase.CommittedUnversioned, metabase.CommittedVersioned,
				metabase.DeleteMarkerVersioned, metabase.DeleteMarkerUnversioned:
				return "ObjectRemoved:Delete"
			}
		}
	}
	return ""
}

func extractString(values map[string]interface{}, key string) (string, bool) {
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

func extractInt64(values map[string]interface{}, key string) (int64, bool) {
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
