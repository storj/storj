// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
)

var (
	TestProjectID = uuid.UUID([16]byte{0xd0, 0xfe, 0xe6, 0xc4, 0x12, 0x37, 0x42, 0x24, 0x96, 0x48, 0xcf, 0xab, 0xe3, 0x1f, 0x6e, 0x6f})
	TestBucket    = "bucket1"
	TestStreamID  = uuid.UUID([16]byte{0x93, 0x72, 0x6b, 0x8d, 0xd0, 0x4a, 0x45, 0xbb, 0x82, 0x4f, 0x67, 0x31, 0x86, 0xee, 0x6f, 0x96})
)

func testStreamVersionID(version int64) string {
	return hex.EncodeToString(metabase.NewStreamVersionID(metabase.Version(version), TestStreamID).Bytes())
}

func TestConvertModsToEvent_Delete(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/delete.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Len(t, event.Records, 1)
	record := event.Records[0]
	assert.Equal(t, "2.1", record.EventVersion)
	assert.Equal(t, "storj:s3", record.EventSource)
	assert.Equal(t, "2025-09-03T08:39:00.349Z", record.EventTime)
	assert.Equal(t, S3ObjectRemovedDelete, record.EventName)
	assert.Equal(t, "1.0", record.S3.S3SchemaVersion)
	assert.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
	assert.Equal(t, TestBucket, record.S3.Bucket.Name)
	assert.Equal(t, "arn:storj:s3:::"+TestBucket, record.S3.Bucket.Arn)
	assert.Equal(t, TestProjectID.String(), record.S3.Bucket.OwnerIdentity.PrincipalId)
	assert.Equal(t, "object1", record.S3.Object.Key)
	assert.Equal(t, testStreamVersionID(99), record.S3.Object.VersionId)
	assert.Equal(t, int64(4194304), record.S3.Object.Size)
	assert.Equal(t, "1861B9003E6CD718", record.S3.Object.Sequencer)
}

func TestConvertModsToEvent_Insert(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/insert.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Len(t, event.Records, 1)
	record := event.Records[0]
	assert.Equal(t, "2.1", record.EventVersion)
	assert.Equal(t, "storj:s3", record.EventSource)
	assert.Equal(t, "2025-09-03T08:39:00.349Z", record.EventTime)
	assert.Equal(t, S3ObjectCreatedPut, record.EventName)
	assert.Equal(t, "1.0", record.S3.S3SchemaVersion)
	assert.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
	assert.Equal(t, TestBucket, record.S3.Bucket.Name)
	assert.Equal(t, "arn:storj:s3:::"+TestBucket, record.S3.Bucket.Arn)
	assert.Equal(t, TestProjectID.String(), record.S3.Bucket.OwnerIdentity.PrincipalId)
	assert.Equal(t, "object1", record.S3.Object.Key)
	assert.Equal(t, testStreamVersionID(100), record.S3.Object.VersionId)
	assert.Equal(t, int64(4194304), record.S3.Object.Size)
	assert.Equal(t, "1861B9003E6CD718", record.S3.Object.Sequencer)
}

func TestConvertModsToEvent_Update(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/update.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Len(t, event.Records, 1)
	record := event.Records[0]
	assert.Equal(t, "2.1", record.EventVersion)
	assert.Equal(t, "storj:s3", record.EventSource)
	assert.Equal(t, "2025-09-24T08:54:41.183Z", record.EventTime)
	assert.Equal(t, S3ObjectCreatedPut, record.EventName)
	assert.Equal(t, "1.0", record.S3.S3SchemaVersion)
	assert.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
	assert.Equal(t, TestBucket, record.S3.Bucket.Name)
	assert.Equal(t, "arn:storj:s3:::"+TestBucket, record.S3.Bucket.Arn)
	assert.Equal(t, TestProjectID.String(), record.S3.Bucket.OwnerIdentity.PrincipalId)
	assert.Equal(t, "object1", record.S3.Object.Key)
	assert.Equal(t, testStreamVersionID(100), record.S3.Object.VersionId)
	assert.Equal(t, int64(123), record.S3.Object.Size)
	assert.Equal(t, "18682C0B37F170B8", record.S3.Object.Sequencer)
}

func TestConvertModsToEvent(t *testing.T) {
	testTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("ObjectCreated:Put for CommittedUnversioned", func(t *testing.T) {
		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*changestream.Mods{
				{
					NewValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"stream_id":        "k3JrjdBKRbuCT2cxhu5vlg==", // base64: TestStreamID
							"status":           float64(3),                 // CommittedUnversioned
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
		assert.Equal(t, "2.1", record.EventVersion)
		assert.Equal(t, "storj:s3", record.EventSource)
		assert.Equal(t, "2024-01-01T12:00:00.000Z", record.EventTime)
		assert.Equal(t, S3ObjectCreatedPut, record.EventName)
		assert.Equal(t, "1.0", record.S3.S3SchemaVersion)
		assert.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
		assert.Equal(t, "test-bucket", record.S3.Bucket.Name)
		assert.Equal(t, "arn:storj:s3:::test-bucket", record.S3.Bucket.Arn)
		assert.Equal(t, "test/object.txt", record.S3.Object.Key)
		assert.Equal(t, int64(1024), record.S3.Object.Size)
		assert.Equal(t, testStreamVersionID(1), record.S3.Object.VersionId)
		assert.Equal(t, fmt.Sprintf("%016X", testTime.UnixNano()), record.S3.Object.Sequencer)
	})

	t.Run("ObjectCreated:Put for CommittedVersioned", func(t *testing.T) {
		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*changestream.Mods{
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
							"stream_id":        "k3JrjdBKRbuCT2cxhu5vlg==", // base64: TestStreamID
							"status":           float64(4),                 // CommittedVersioned
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
		assert.Equal(t, S3ObjectCreatedPut, record.EventName)
		assert.Equal(t, int64(2048), record.S3.Object.Size)
	})

	t.Run("ObjectCreated:Put for CommittedUnversioned with update", func(t *testing.T) {
		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "UPDATE",
			Mods: []*changestream.Mods{
				{
					NewValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"stream_id":        "k3JrjdBKRbuCT2cxhu5vlg==", // base64: TestStreamID
							"status":           float64(3),                 // CommittedUnversioned
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
		assert.Equal(t, "2.1", record.EventVersion)
		assert.Equal(t, "storj:s3", record.EventSource)
		assert.Equal(t, "2024-01-01T12:00:00.000Z", record.EventTime)
		assert.Equal(t, S3ObjectCreatedPut, record.EventName)
		assert.Equal(t, "1.0", record.S3.S3SchemaVersion)
		assert.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
		assert.Equal(t, "test-bucket", record.S3.Bucket.Name)
		assert.Equal(t, "arn:storj:s3:::test-bucket", record.S3.Bucket.Arn)
		assert.Equal(t, "test/object.txt", record.S3.Object.Key)
		assert.Equal(t, int64(1024), record.S3.Object.Size)
		assert.Equal(t, testStreamVersionID(1), record.S3.Object.VersionId)
		assert.Equal(t, fmt.Sprintf("%016X", testTime.UnixNano()), record.S3.Object.Sequencer)
	})

	t.Run("ObjectCreated:Put for CommittedVersioned with update", func(t *testing.T) {
		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "UPDATE",
			Mods: []*changestream.Mods{
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
		assert.Equal(t, S3ObjectCreatedPut, record.EventName)
		assert.Equal(t, int64(2048), record.S3.Object.Size)
	})

	t.Run("ObjectRemoved:DeleteMarkerCreated for DeleteMarkerVersioned", func(t *testing.T) {
		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*changestream.Mods{
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
							"stream_id": "k3JrjdBKRbuCT2cxhu5vlg==", // base64: TestStreamID
							"status":    float64(5),                 // DeleteMarkerVersioned
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
		require.Equal(t, S3ObjectRemovedDeleteMarkerCreated, record.EventName)
		assert.Equal(t, "test/deleted-object.txt", record.S3.Object.Key)
		assert.Equal(t, testStreamVersionID(3), record.S3.Object.VersionId)
	})

	t.Run("ObjectRemoved:DeleteMarkerCreated for DeleteMarkerUnversioned", func(t *testing.T) {
		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*changestream.Mods{
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
		assert.Equal(t, S3ObjectRemovedDeleteMarkerCreated, record.EventName)
	})

	t.Run("ObjectRemoved:Delete for DELETE operation", func(t *testing.T) {
		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "DELETE",
			Mods: []*changestream.Mods{
				{
					OldValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"stream_id": "k3JrjdBKRbuCT2cxhu5vlg==", // base64: TestStreamID
							"status":    float64(3),                 // CommittedUnversioned
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
		assert.Equal(t, S3ObjectRemovedDelete, record.EventName)
		assert.Equal(t, "test-bucket", record.S3.Bucket.Name)
		assert.Equal(t, "test/deleted-object.txt", record.S3.Object.Key)
		assert.Equal(t, testStreamVersionID(1), record.S3.Object.VersionId)
	})

	t.Run("Multiple mods in single record", func(t *testing.T) {
		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*changestream.Mods{
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
							"bucket_name": TestBucket,
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

		assert.Equal(t, TestBucket, event.Records[0].S3.Bucket.Name)
		assert.Equal(t, "object1.txt", event.Records[0].S3.Object.Key)
		assert.Equal(t, "bucket2", event.Records[1].S3.Bucket.Name)
		assert.Equal(t, "object2.txt", event.Records[1].S3.Object.Key)
	})

	t.Run("Invalid JSON in NewValues returns error", func(t *testing.T) {
		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*changestream.Mods{
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
		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "DELETE",
			Mods: []*changestream.Mods{
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

		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*changestream.Mods{
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
		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			ModType:         "INSERT",
			Mods: []*changestream.Mods{
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
		require.Equal(t, S3ObjectCreatedPut, eventName)
	})

	t.Run("INSERT with CommittedVersioned", func(t *testing.T) {
		newValues := map[string]interface{}{"status": float64(4)}
		eventName := determineEventName("INSERT", newValues, nil)
		require.Equal(t, S3ObjectCreatedPut, eventName)
	})

	t.Run("UPDATE with CommittedUnversioned", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(1)}
		newValues := map[string]interface{}{"status": float64(3)}
		eventName := determineEventName("UPDATE", newValues, oldValues)
		require.Equal(t, S3ObjectCreatedPut, eventName)
	})

	t.Run("UPDATE with CommittedVersioned", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(1)}
		newValues := map[string]interface{}{"status": float64(4)}
		eventName := determineEventName("UPDATE", newValues, oldValues)
		require.Equal(t, S3ObjectCreatedPut, eventName)
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
		require.Equal(t, S3ObjectRemovedDeleteMarkerCreated, eventName)
	})

	t.Run("INSERT with DeleteMarkerUnversioned", func(t *testing.T) {
		newValues := map[string]interface{}{"status": float64(6)}
		eventName := determineEventName("INSERT", newValues, nil)
		require.Equal(t, S3ObjectRemovedDeleteMarkerCreated, eventName)
	})

	t.Run("DELETE with CommittedUnversioned", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(3)}
		eventName := determineEventName("DELETE", nil, oldValues)
		require.Equal(t, S3ObjectRemovedDelete, eventName)
	})

	t.Run("DELETE with CommittedVersioned", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(4)}
		eventName := determineEventName("DELETE", nil, oldValues)
		require.Equal(t, S3ObjectRemovedDelete, eventName)
	})

	t.Run("DELETE with DeleteMarkerVersioned", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(5)}
		eventName := determineEventName("DELETE", nil, oldValues)
		require.Equal(t, S3ObjectRemovedDelete, eventName)
	})

	t.Run("DELETE with DeleteMarkerUnversioned", func(t *testing.T) {
		oldValues := map[string]interface{}{"status": float64(6)}
		eventName := determineEventName("DELETE", nil, oldValues)
		require.Equal(t, S3ObjectRemovedDelete, eventName)
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
		result, ok := extractString("string_key", values)
		require.True(t, ok)
		require.Equal(t, "test_value", result)
	})

	t.Run("Non-string value", func(t *testing.T) {
		result, ok := extractString("int_key", values)
		require.False(t, ok)
		require.Equal(t, "", result)
	})

	t.Run("Missing key", func(t *testing.T) {
		result, ok := extractString("missing_key", values)
		require.False(t, ok)
		require.Equal(t, "", result)
	})

	t.Run("Nil values map", func(t *testing.T) {
		result, ok := extractString("any_key", nil)
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
		result, ok := extractInt64("int64_key", values)
		require.True(t, ok)
		require.Equal(t, int64(123), result)
	})

	t.Run("Valid float64 extraction", func(t *testing.T) {
		result, ok := extractInt64("float64_key", values)
		require.True(t, ok)
		require.Equal(t, int64(456), result)
	})

	t.Run("Valid json.Number extraction", func(t *testing.T) {
		result, ok := extractInt64("json_num_key", values)
		require.True(t, ok)
		require.Equal(t, int64(789), result)
	})

	t.Run("Non-numeric value", func(t *testing.T) {
		result, ok := extractInt64("string_key", values)
		require.False(t, ok)
		require.Equal(t, int64(0), result)
	})

	t.Run("Missing key", func(t *testing.T) {
		result, ok := extractInt64("missing_key", values)
		require.False(t, ok)
		require.Equal(t, int64(0), result)
	})

	t.Run("Nil values map", func(t *testing.T) {
		result, ok := extractInt64("any_key", nil)
		require.False(t, ok)
		require.Equal(t, int64(0), result)
	})
}

func TestExtractFirst(t *testing.T) {
	values1 := map[string]interface{}{
		"string_key": "first_value",
		"int64_key":  int64(123),
		"nil_key":    nil,
	}

	values2 := map[string]interface{}{
		"string_key": "second_value",
		"int64_key":  int64(321),
		"nil_key":    nil,
	}

	t.Run("Valid string extraction", func(t *testing.T) {
		result, ok := extractFirstString("string_key", values1, values2)
		require.True(t, ok)
		require.Equal(t, "first_value", result)
	})

	t.Run("Valid string extraction, first nil map", func(t *testing.T) {
		result, ok := extractFirstString("string_key", nil, values2)
		require.True(t, ok)
		require.Equal(t, "second_value", result)
	})

	t.Run("Valid int64 extraction", func(t *testing.T) {
		result, ok := extractFirstInt64("int64_key", values1, values2)
		require.True(t, ok)
		require.Equal(t, int64(123), result)
	})

	t.Run("Valid int64 extraction, first nil map", func(t *testing.T) {
		result, ok := extractFirstInt64("int64_key", nil, values2)
		require.True(t, ok)
		require.Equal(t, int64(321), result)
	})

	t.Run("Nil values map, string", func(t *testing.T) {
		result, ok := extractFirstString("any_key", nil, nil)
		require.False(t, ok)
		require.Equal(t, "", result)
	})

	t.Run("Nil values map, int64", func(t *testing.T) {
		result, ok := extractFirstInt64("any_key", nil, nil)
		require.False(t, ok)
		require.Equal(t, int64(0), result)
	})
}
