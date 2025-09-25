// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/require"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

var (
	TestBucket = metabase.BucketLocation{
		ProjectID:  uuid.UUID([16]byte{0xd0, 0xfe, 0xe6, 0xc4, 0x12, 0x37, 0x42, 0x24, 0x96, 0x48, 0xcf, 0xab, 0xe3, 0x1f, 0x6e, 0x6f}),
		BucketName: metabase.BucketName("bucket1"),
	}
)

func TestConvertModsToEvent_Delete(t *testing.T) {
	var r metabase.DataChangeRecord
	raw, err := os.ReadFile("./testdata/delete.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Equal(t, TestBucket, event.Bucket)
	require.Len(t, event.Records, 1)
	record := event.Records[0]
	require.Equal(t, "2.1", record.EventVersion)
	require.Equal(t, "storj:s3", record.EventSource)
	require.Equal(t, "2025-09-03T08:39:00.349Z", record.EventTime)
	require.Equal(t, "ObjectRemoved:Delete", record.EventName)
	require.Equal(t, "1.0", record.S3.S3SchemaVersion)
	require.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
	require.Equal(t, "bucket1", record.S3.Bucket.Name)
	require.Equal(t, "arn:storj:s3:::bucket1", record.S3.Bucket.Arn)
	require.Equal(t, "object1", record.S3.Object.Key)
	require.Equal(t, "99", record.S3.Object.VersionId)
	require.Equal(t, "1861B9003E6CD718", record.S3.Object.Sequencer)
}

func TestConvertModsToEvent_Insert(t *testing.T) {
	var r metabase.DataChangeRecord
	raw, err := os.ReadFile("./testdata/insert.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Equal(t, TestBucket, event.Bucket)
	require.Len(t, event.Records, 1)
	record := event.Records[0]
	require.Equal(t, "2.1", record.EventVersion)
	require.Equal(t, "storj:s3", record.EventSource)
	require.Equal(t, "2025-09-03T08:39:00.349Z", record.EventTime)
	require.Equal(t, "ObjectCreated:Put", record.EventName)
	require.Equal(t, "1.0", record.S3.S3SchemaVersion)
	require.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
	require.Equal(t, "bucket1", record.S3.Bucket.Name)
	require.Equal(t, "arn:storj:s3:::bucket1", record.S3.Bucket.Arn)
	require.Equal(t, "object1", record.S3.Object.Key)
	require.Equal(t, "100", record.S3.Object.VersionId)
	require.Equal(t, "1861B9003E6CD718", record.S3.Object.Sequencer)
}

func TestConvertModsToEvent_Update(t *testing.T) {
	var r metabase.DataChangeRecord
	raw, err := os.ReadFile("./testdata/update.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Equal(t, TestBucket, event.Bucket)
	require.Len(t, event.Records, 1)
	record := event.Records[0]
	require.Equal(t, "2.1", record.EventVersion)
	require.Equal(t, "storj:s3", record.EventSource)
	require.Equal(t, "2025-09-24T08:54:41.183Z", record.EventTime)
	require.Equal(t, "ObjectCreated:Put", record.EventName)
	require.Equal(t, "1.0", record.S3.S3SchemaVersion)
	require.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
	require.Equal(t, "bucket1", record.S3.Bucket.Name)
	require.Equal(t, "arn:storj:s3:::bucket1", record.S3.Bucket.Arn)
	require.Equal(t, "object1", record.S3.Object.Key)
	require.Equal(t, "100", record.S3.Object.VersionId)
	require.Equal(t, "18682C0B37F170B8", record.S3.Object.Sequencer)
}

func TestConvertModsToEvent(t *testing.T) {
	testTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("ObjectCreated:Put for CommittedUnversioned", func(t *testing.T) {
		dataRecord := metabase.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*metabase.Mods{
				{
					NewValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"status":           float64(3), // CommittedUnversioned
							"total_plain_size": float64(1024),
						},
						Valid: true,
					},
					Keys: spanner.NullJSON{
						Value: map[string]interface{}{
							"bucket_name": "test-bucket",
							"object_key":  "dGVzdC9vYmplY3QudHh0", // base64: test/object.txt
							"version":     float64(1),
						},
						Valid: true,
					},
				},
			},
		}

		event, err := ConvertModsToEvent(dataRecord)
		require.NoError(t, err)
		require.Len(t, event.Records, 1)

		record := event.Records[0]
		require.Equal(t, "2.1", record.EventVersion)
		require.Equal(t, "storj:s3", record.EventSource)
		require.Equal(t, "2024-01-01T12:00:00.000Z", record.EventTime)
		require.Equal(t, "ObjectCreated:Put", record.EventName)
		require.Equal(t, "1.0", record.S3.S3SchemaVersion)
		require.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
		require.Equal(t, "test-bucket", record.S3.Bucket.Name)
		require.Equal(t, "arn:storj:s3:::test-bucket", record.S3.Bucket.Arn)
		require.Equal(t, "test/object.txt", record.S3.Object.Key)
		require.Equal(t, int64(1024), record.S3.Object.Size)
		require.Equal(t, "1", record.S3.Object.VersionId)
		require.Equal(t, fmt.Sprintf("%016X", testTime.UnixNano()), record.S3.Object.Sequencer)
	})

	t.Run("ObjectCreated:Put for CommittedVersioned", func(t *testing.T) {
		dataRecord := metabase.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*metabase.Mods{
				{
					Keys: spanner.NullJSON{
						Value: map[string]interface{}{
							"bucket_name": "test-bucket",
							"object_key":  "dGVzdC9vYmplY3QudHh0", // base64: test/object.txt
							"version":     float64(2),
						},
						Valid: true,
					},
					NewValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"status":           float64(4), // CommittedVersioned
							"total_plain_size": float64(2048),
						},
						Valid: true,
					},
				},
			},
		}

		event, err := ConvertModsToEvent(dataRecord)
		require.NoError(t, err)
		require.Len(t, event.Records, 1)

		record := event.Records[0]
		require.Equal(t, "ObjectCreated:Put", record.EventName)
		require.Equal(t, int64(2048), record.S3.Object.Size)
	})

	t.Run("ObjectCreated:Put for CommittedUnversioned with update", func(t *testing.T) {
		dataRecord := metabase.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "UPDATE",
			Mods: []*metabase.Mods{
				{
					NewValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"status":           float64(3), // CommittedUnversioned
							"total_plain_size": float64(1024),
						},
						Valid: true,
					},
					Keys: spanner.NullJSON{
						Value: map[string]interface{}{
							"bucket_name": "test-bucket",
							"object_key":  "dGVzdC9vYmplY3QudHh0", // base64: test/object.txt
							"version":     float64(1),
						},
						Valid: true,
					},
					OldValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"status":           float64(1), // Pending
							"total_plain_size": float64(0),
						},
						Valid: true,
					},
				},
			},
		}

		event, err := ConvertModsToEvent(dataRecord)
		require.NoError(t, err)
		require.Len(t, event.Records, 1)

		record := event.Records[0]
		require.Equal(t, "2.1", record.EventVersion)
		require.Equal(t, "storj:s3", record.EventSource)
		require.Equal(t, "2024-01-01T12:00:00.000Z", record.EventTime)
		require.Equal(t, "ObjectCreated:Put", record.EventName)
		require.Equal(t, "1.0", record.S3.S3SchemaVersion)
		require.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
		require.Equal(t, "test-bucket", record.S3.Bucket.Name)
		require.Equal(t, "arn:storj:s3:::test-bucket", record.S3.Bucket.Arn)
		require.Equal(t, "test/object.txt", record.S3.Object.Key)
		require.Equal(t, int64(1024), record.S3.Object.Size)
		require.Equal(t, "1", record.S3.Object.VersionId)
		require.Equal(t, fmt.Sprintf("%016X", testTime.UnixNano()), record.S3.Object.Sequencer)
	})

	t.Run("ObjectCreated:Put for CommittedVersioned with update", func(t *testing.T) {
		dataRecord := metabase.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "UPDATE",
			Mods: []*metabase.Mods{
				{
					Keys: spanner.NullJSON{
						Value: map[string]interface{}{
							"bucket_name": "test-bucket",
							"object_key":  "dGVzdC9vYmplY3QudHh0", // base64: test/object.txt
							"version":     float64(2),
						},
						Valid: true,
					},
					NewValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"status":           float64(4), // CommittedVersioned
							"total_plain_size": float64(2048),
						},
						Valid: true,
					},
					OldValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"status":           float64(1), // Pending
							"total_plain_size": float64(0),
						},
						Valid: true,
					},
				},
			},
		}

		event, err := ConvertModsToEvent(dataRecord)
		require.NoError(t, err)
		require.Len(t, event.Records, 1)

		record := event.Records[0]
		require.Equal(t, "ObjectCreated:Put", record.EventName)
		require.Equal(t, int64(2048), record.S3.Object.Size)
	})

	t.Run("ObjectRemoved:DeleteMarkerCreated for DeleteMarkerVersioned", func(t *testing.T) {
		dataRecord := metabase.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*metabase.Mods{
				{
					Keys: spanner.NullJSON{
						Value: map[string]interface{}{
							"bucket_name": "test-bucket",
							"object_key":  "dGVzdC9kZWxldGVkLW9iamVjdC50eHQ=", // base64: test/deleted-object.txt
							"version":     float64(3),
						},
						Valid: true,
					},
					NewValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"status": float64(5), // DeleteMarkerVersioned
						},
						Valid: true,
					},
				},
			},
		}

		event, err := ConvertModsToEvent(dataRecord)
		require.NoError(t, err)
		require.Len(t, event.Records, 1)

		record := event.Records[0]
		require.Equal(t, "ObjectRemoved:DeleteMarkerCreated", record.EventName)
		require.Equal(t, "test/deleted-object.txt", record.S3.Object.Key)
		require.Equal(t, "3", record.S3.Object.VersionId)
	})

	t.Run("ObjectRemoved:DeleteMarkerCreated for DeleteMarkerUnversioned", func(t *testing.T) {
		dataRecord := metabase.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*metabase.Mods{
				{
					NewValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"status": float64(6), // DeleteMarkerUnversioned
						},
						Valid: true,
					},
					Keys: spanner.NullJSON{
						Value: map[string]interface{}{
							"bucket_name": "test-bucket",
							"object_key":  "dGVzdC9kZWxldGVkLW9iamVjdC50eHQ=", // base64: test/deleted-object.txt
						},
						Valid: true,
					},
				},
			},
		}

		event, err := ConvertModsToEvent(dataRecord)
		require.NoError(t, err)
		require.Len(t, event.Records, 1)

		record := event.Records[0]
		require.Equal(t, "ObjectRemoved:DeleteMarkerCreated", record.EventName)
	})

	t.Run("ObjectRemoved:Delete for DELETE operation", func(t *testing.T) {
		dataRecord := metabase.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "DELETE",
			Mods: []*metabase.Mods{
				{
					OldValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"status": float64(3), // CommittedUnversioned
						},
						Valid: true,
					},
					Keys: spanner.NullJSON{
						Value: map[string]interface{}{
							"bucket_name": "test-bucket",
							"object_key":  "dGVzdC9kZWxldGVkLW9iamVjdC50eHQ=", // base64: test/deleted-object.txt
							"version":     float64(1),
						},
						Valid: true,
					},
				},
			},
		}

		event, err := ConvertModsToEvent(dataRecord)
		require.NoError(t, err)
		require.Len(t, event.Records, 1)

		record := event.Records[0]
		require.Equal(t, "ObjectRemoved:Delete", record.EventName)
		require.Equal(t, "test-bucket", record.S3.Bucket.Name)
		require.Equal(t, "test/deleted-object.txt", record.S3.Object.Key)
		require.Equal(t, "1", record.S3.Object.VersionId)
	})

	t.Run("Multiple mods in single record", func(t *testing.T) {
		dataRecord := metabase.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*metabase.Mods{
				{
					NewValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"status":           float64(3),
							"total_plain_size": float64(100),
						},
						Valid: true,
					},
					Keys: spanner.NullJSON{
						Value: map[string]interface{}{
							"bucket_name": "bucket1",
							"object_key":  "b2JqZWN0MS50eHQ=", // base64: object1.txt
							"version":     float64(1),
						},
						Valid: true,
					},
				},
				{
					NewValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"status":           float64(4),
							"total_plain_size": float64(200),
						},
						Valid: true,
					},
					Keys: spanner.NullJSON{
						Value: map[string]interface{}{
							"bucket_name": "bucket2",
							"object_key":  "b2JqZWN0Mi50eHQ=", // base64: object2.txt
							"version":     float64(2),
						},
						Valid: true,
					},
				},
			},
		}

		event, err := ConvertModsToEvent(dataRecord)
		require.NoError(t, err)
		require.Len(t, event.Records, 2)

		require.Equal(t, "bucket1", event.Records[0].S3.Bucket.Name)
		require.Equal(t, "object1.txt", event.Records[0].S3.Object.Key)
		require.Equal(t, "bucket2", event.Records[1].S3.Bucket.Name)
		require.Equal(t, "object2.txt", event.Records[1].S3.Object.Key)
	})

	t.Run("Invalid JSON in NewValues returns error", func(t *testing.T) {
		dataRecord := metabase.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*metabase.Mods{
				{
					NewValues: spanner.NullJSON{Value: "invalid json", Valid: true},
				},
			},
		}

		_, err := ConvertModsToEvent(dataRecord)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal new values")
	})

	t.Run("Invalid JSON in OldValues returns error", func(t *testing.T) {
		dataRecord := metabase.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "DELETE",
			Mods: []*metabase.Mods{
				{
					OldValues: spanner.NullJSON{Value: "invalid json", Valid: true},
				},
			},
		}

		_, err := ConvertModsToEvent(dataRecord)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal old values")
	})

	t.Run("Unknown event type is skipped", func(t *testing.T) {
		newValues := map[string]interface{}{
			"bucket_name": "test-bucket",
			"object_key":  "dGVzdC9vYmplY3QudHh0", // base64: test/object.txt
			"status":      float64(1),             // Pending - should be skipped
		}
		newValuesJSON, _ := json.Marshal(newValues)

		dataRecord := metabase.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*metabase.Mods{
				{
					NewValues: spanner.NullJSON{Value: string(newValuesJSON), Valid: true},
				},
			},
		}

		event, err := ConvertModsToEvent(dataRecord)
		require.NoError(t, err)
		require.Len(t, event.Records, 0)
	})

	t.Run("No valid JSON values", func(t *testing.T) {
		dataRecord := metabase.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*metabase.Mods{
				{
					NewValues: spanner.NullJSON{Valid: false},
					OldValues: spanner.NullJSON{Valid: false},
				},
			},
		}

		event, err := ConvertModsToEvent(dataRecord)
		require.NoError(t, err)
		require.Len(t, event.Records, 0)
	})
}

func TestDetermineEventName(t *testing.T) {
	t.Run("INSERT with CommittedUnversioned", func(t *testing.T) {
		newValues := map[string]interface{}{"status": float64(3)}
		eventName := determineEventName("INSERT", newValues, nil)
		require.Equal(t, "ObjectCreated:Put", eventName)
	})

	t.Run("INSERT with CommittedVersioned", func(t *testing.T) {
		newValues := map[string]interface{}{"status": float64(4)}
		eventName := determineEventName("INSERT", newValues, nil)
		require.Equal(t, "ObjectCreated:Put", eventName)
	})

	t.Run("UPDATE with CommittedUnversioned", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(1)}
		newValues := map[string]interface{}{"status": float64(3)}
		eventName := determineEventName("UPDATE", newValues, oldValues)
		require.Equal(t, "ObjectCreated:Put", eventName)
	})

	t.Run("UPDATE with CommittedVersioned", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(1)}
		newValues := map[string]interface{}{"status": float64(4)}
		eventName := determineEventName("UPDATE", newValues, oldValues)
		require.Equal(t, "ObjectCreated:Put", eventName)
	})

	t.Run("UPDATE with no pending status change returns empty", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(3)}
		newValues := map[string]interface{}{"status": float64(3)}
		eventName := determineEventName("UPDATE", newValues, oldValues)
		require.Equal(t, "", eventName)
	})

	t.Run("UPDATE with no committed status change returns empty", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(1)}
		newValues := map[string]interface{}{"status": float64(1)}
		eventName := determineEventName("UPDATE", newValues, oldValues)
		require.Equal(t, "", eventName)
	})

	t.Run("UPDATE with no committed status change and no old values", func(t *testing.T) {
		newValues := map[string]interface{}{"status": float64(1)}
		eventName := determineEventName("UPDATE", newValues, nil)
		require.Equal(t, "", eventName)
	})

	t.Run("INSERT with DeleteMarkerVersioned", func(t *testing.T) {
		newValues := map[string]interface{}{"status": float64(5)}
		eventName := determineEventName("INSERT", newValues, nil)
		require.Equal(t, "ObjectRemoved:DeleteMarkerCreated", eventName)
	})

	t.Run("INSERT with DeleteMarkerUnversioned", func(t *testing.T) {
		newValues := map[string]interface{}{"status": float64(6)}
		eventName := determineEventName("INSERT", newValues, nil)
		require.Equal(t, "ObjectRemoved:DeleteMarkerCreated", eventName)
	})

	t.Run("DELETE with CommittedUnversioned", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(3)}
		eventName := determineEventName("DELETE", nil, oldValues)
		require.Equal(t, "ObjectRemoved:Delete", eventName)
	})

	t.Run("DELETE with CommittedVersioned", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(4)}
		eventName := determineEventName("DELETE", nil, oldValues)
		require.Equal(t, "ObjectRemoved:Delete", eventName)
	})

	t.Run("DELETE with DeleteMarkerVersioned", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(5)}
		eventName := determineEventName("DELETE", nil, oldValues)
		require.Equal(t, "ObjectRemoved:Delete", eventName)
	})

	t.Run("DELETE with DeleteMarkerUnversioned", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(6)}
		eventName := determineEventName("DELETE", nil, oldValues)
		require.Equal(t, "ObjectRemoved:Delete", eventName)
	})

	t.Run("Unknown mod type returns empty", func(t *testing.T) {
		newValues := map[string]interface{}{"status": float64(3)}
		eventName := determineEventName("UPDATE", newValues, nil)
		require.Equal(t, "", eventName)
	})

	t.Run("INSERT with Pending status returns empty", func(t *testing.T) {
		newValues := map[string]interface{}{"status": float64(1)} // Pending
		eventName := determineEventName("INSERT", newValues, nil)
		require.Equal(t, "", eventName)
	})
}

func TestExtractString(t *testing.T) {
	values := map[string]interface{}{
		"string_key": "test_value",
		"int_key":    123,
		"nil_key":    nil,
	}

	t.Run("Valid string extraction", func(t *testing.T) {
		result, ok := extractString(values, "string_key")
		require.True(t, ok)
		require.Equal(t, "test_value", result)
	})

	t.Run("Non-string value", func(t *testing.T) {
		result, ok := extractString(values, "int_key")
		require.False(t, ok)
		require.Equal(t, "", result)
	})

	t.Run("Missing key", func(t *testing.T) {
		result, ok := extractString(values, "missing_key")
		require.False(t, ok)
		require.Equal(t, "", result)
	})

	t.Run("Nil values map", func(t *testing.T) {
		result, ok := extractString(nil, "any_key")
		require.False(t, ok)
		require.Equal(t, "", result)
	})
}

func TestExtractInt64(t *testing.T) {
	values := map[string]interface{}{
		"int64_key":    int64(123),
		"float64_key":  float64(456.0),
		"string_key":   "not_a_number",
		"json_num_key": json.Number("789"),
	}

	t.Run("Valid int64 extraction", func(t *testing.T) {
		result, ok := extractInt64(values, "int64_key")
		require.True(t, ok)
		require.Equal(t, int64(123), result)
	})

	t.Run("Valid float64 extraction", func(t *testing.T) {
		result, ok := extractInt64(values, "float64_key")
		require.True(t, ok)
		require.Equal(t, int64(456), result)
	})

	t.Run("Valid json.Number extraction", func(t *testing.T) {
		result, ok := extractInt64(values, "json_num_key")
		require.True(t, ok)
		require.Equal(t, int64(789), result)
	})

	t.Run("Non-numeric value", func(t *testing.T) {
		result, ok := extractInt64(values, "string_key")
		require.False(t, ok)
		require.Equal(t, int64(0), result)
	})

	t.Run("Missing key", func(t *testing.T) {
		result, ok := extractInt64(values, "missing_key")
		require.False(t, ok)
		require.Equal(t, int64(0), result)
	})

	t.Run("Nil values map", func(t *testing.T) {
		result, ok := extractInt64(nil, "any_key")
		require.False(t, ok)
		require.Equal(t, int64(0), result)
	})
}
