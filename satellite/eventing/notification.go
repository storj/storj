// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// ErrInvalidEventType is used when an invalid bucket event type is encountered.
var ErrInvalidEventType = errs.Class("invalid bucket event type")

const (
	eventCategoryObjectCreated = "ObjectCreated"
	eventCategoryObjectRemoved = "ObjectRemoved"
)

// Event names (without "s3:" prefix) used in the EventName field of published event records.
const (
	EventNameObjectCreatedPut                 = eventCategoryObjectCreated + ":Put"
	EventNameObjectCreatedCopy                = eventCategoryObjectCreated + ":Copy"
	EventNameObjectRemovedDelete              = eventCategoryObjectRemoved + ":Delete"
	EventNameObjectRemovedDeleteMarkerCreated = eventCategoryObjectRemoved + ":DeleteMarkerCreated"
)

// Event types (with "s3:" prefix) used in bucket notification configuration.
const (
	EventTypeObjectCreatedPut                 = "s3:" + EventNameObjectCreatedPut
	EventTypeObjectCreatedCopy                = "s3:" + EventNameObjectCreatedCopy
	EventTypeObjectRemovedDelete              = "s3:" + EventNameObjectRemovedDelete
	EventTypeObjectRemovedDeleteMarkerCreated = "s3:" + EventNameObjectRemovedDeleteMarkerCreated
	EventTypeObjectCreatedAll                 = "s3:" + eventCategoryObjectCreated + ":*"
	EventTypeObjectRemovedAll                 = "s3:" + eventCategoryObjectRemoved + ":*"
)

// allowedEvents is the set of valid event types.
var allowedEvents = map[string]bool{
	EventTypeObjectCreatedPut:                 true,
	EventTypeObjectCreatedCopy:                true,
	EventTypeObjectRemovedDelete:              true,
	EventTypeObjectRemovedDeleteMarkerCreated: true,
	EventTypeObjectCreatedAll:                 true,
	EventTypeObjectRemovedAll:                 true,
}

// ISO8601 is the time format used in S3 event notifications.
const ISO8601 = "2006-01-02T15:04:05.000Z"

// TestEvent represents an S3 test event sent to validate topic accessibility.
type TestEvent struct {
	Service string `json:"Service"`
	Event   string `json:"Event"`
	Time    string `json:"Time"`
	Bucket  string `json:"Bucket"`
}

// Event contains one or more event records.
type Event struct {
	Records []EventRecord `json:"Records,omitempty"`
}

// Bytes returns the JSON-encoded representation of the event.
func (e Event) Bytes() ([]byte, error) {
	return json.Marshal(e)
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

// buildS3Event constructs an S3-compatible Event from a ChangeEvent.
// The project ID must already be the public project ID.
func buildS3Event(event ChangeEvent, projectPublicID uuid.UUID, configID string) Event {
	record := EventRecord{}
	record.EventVersion = "2.1"
	record.EventSource = "storj:s3"
	record.EventTime = event.CommitTimestamp.UTC().Format(ISO8601)
	record.S3.S3SchemaVersion = "1.0"
	record.EventName = event.EventName
	record.S3.ConfigurationId = configID

	bucketName := string(event.BucketName)
	record.S3.Bucket.Name = bucketName
	record.S3.Bucket.Arn = fmt.Sprintf("arn:storj:s3:::%s", bucketName)
	record.S3.Bucket.OwnerIdentity.PrincipalId = projectPublicID.String()

	// The object key is URL-encoded per the S3 spec.
	record.S3.Object.Key = string(EncodeForS3Event([]byte(event.ObjectKey)))
	record.S3.Object.Size = event.TotalPlainSize

	streamVersionID := metabase.NewStreamVersionID(event.Version, event.StreamID)
	record.S3.Object.VersionId = hex.EncodeToString(streamVersionID.Bytes())
	record.S3.Object.Sequencer = fmt.Sprintf("%016X", event.CommitTimestamp.UnixNano())

	return Event{Records: []EventRecord{record}}
}

// CreateTestEvent creates an S3-compatible test event for the given bucket.
func CreateTestEvent(bucketName string) TestEvent {
	return TestEvent{
		Service: "Storj S3",
		Event:   "s3:TestEvent",
		Time:    time.Now().UTC().Format(ISO8601),
		Bucket:  bucketName,
	}
}

// Bytes returns the JSON-encoded representation of the test event.
func (e TestEvent) Bytes() ([]byte, error) {
	return json.Marshal(e)
}

// ValidateEventTypes validates that event types are in the allowed list or valid wildcards.
func ValidateEventTypes(events []string) error {
	if len(events) == 0 {
		return errs.New("at least one event type is required")
	}

	for _, event := range events {
		if !allowedEvents[event] {
			return ErrInvalidEventType.New("%s", event)
		}
	}

	return nil
}

// MatchEventType checks if the given event type matches any of the configured event types.
// Supports wildcards like "s3:ObjectCreated:*" and "s3:ObjectRemoved:*".
func MatchEventType(eventType string, configuredEvents []string) bool {
	// Normalize event type by adding s3: prefix if missing
	if !strings.HasPrefix(eventType, "s3:") {
		eventType = "s3:" + eventType
	}

	for _, configuredEvent := range configuredEvents {
		// Exact match
		if configuredEvent == eventType {
			return true
		}

		// Wildcard match
		if prefix, ok := strings.CutSuffix(configuredEvent, "*"); ok {
			if strings.HasPrefix(eventType, prefix) {
				return true
			}
		}
	}

	return false
}

// EncodeForS3Event URL-encodes an S3 object key per the S3 spec: each path segment
// is encoded with query escaping (spaces become "+", special chars become "%XX"),
// while "/" path delimiters are left unencoded.
func EncodeForS3Event(objectKey []byte) []byte {
	segments := strings.Split(string(objectKey), "/")
	for i, s := range segments {
		segments[i] = url.QueryEscape(s)
	}
	return []byte(strings.Join(segments, "/"))
}

// MatchFilters checks if the object key matches the prefix and suffix filters.
func MatchFilters(objectKey []byte, filterPrefix []byte, filterSuffix []byte) bool {
	if len(filterPrefix) > 0 && !bytes.HasPrefix(objectKey, filterPrefix) {
		return false
	}

	if len(filterSuffix) > 0 && !bytes.HasSuffix(objectKey, filterSuffix) {
		return false
	}

	return true
}
