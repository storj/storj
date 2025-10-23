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

func TestConvertModsToEvent_BeginObjectExactVersion(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/begin-object-exact-version.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Empty(t, event.Records)
}

func TestConvertModsToEvent_BeginObjectNextVersion(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/begin-object-next-version.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Empty(t, event.Records)
}

func TestConvertModsToEvent_CommitInlineObject_Delete_Overwrite(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/commit-inline-object-delete-overwrite.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Empty(t, event.Records)
}

func TestConvertModsToEvent_CommitInlineObject_Insert(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/commit-inline-object-insert.json")
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

func TestConvertModsToEvent_CommitObject_Delete(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/commit-object-delete.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Empty(t, event.Records)
}

func TestConvertModsToEvent_CommitObject_Delete_Overwrite(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/commit-object-delete-overwrite.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Empty(t, event.Records)
}

func TestConvertModsToEvent_CommitObject_Insert(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/commit-object-insert.json")
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

func TestConvertModsToEvent_CommitObject_Update(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/commit-object-update.json")
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

func TestConvertModsToEvent_DeleteAllBucketObjects(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/delete-all-bucket-objects.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Len(t, event.Records, 3)
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
	record = event.Records[1]
	assert.Equal(t, "2.1", record.EventVersion)
	assert.Equal(t, "storj:s3", record.EventSource)
	assert.Equal(t, "2025-09-03T08:39:00.349Z", record.EventTime)
	assert.Equal(t, S3ObjectRemovedDelete, record.EventName)
	assert.Equal(t, "1.0", record.S3.S3SchemaVersion)
	assert.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
	assert.Equal(t, TestBucket, record.S3.Bucket.Name)
	assert.Equal(t, "arn:storj:s3:::"+TestBucket, record.S3.Bucket.Arn)
	assert.Equal(t, TestProjectID.String(), record.S3.Bucket.OwnerIdentity.PrincipalId)
	assert.Equal(t, "object2", record.S3.Object.Key)
	assert.Equal(t, testStreamVersionID(100), record.S3.Object.VersionId)
	assert.Equal(t, int64(1024), record.S3.Object.Size)
	assert.Equal(t, "1861B9003E6CD718", record.S3.Object.Sequencer)
	record = event.Records[2]
	assert.Equal(t, "2.1", record.EventVersion)
	assert.Equal(t, "storj:s3", record.EventSource)
	assert.Equal(t, "2025-09-03T08:39:00.349Z", record.EventTime)
	assert.Equal(t, S3ObjectRemovedDelete, record.EventName)
	assert.Equal(t, "1.0", record.S3.S3SchemaVersion)
	assert.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
	assert.Equal(t, TestBucket, record.S3.Bucket.Name)
	assert.Equal(t, "arn:storj:s3:::"+TestBucket, record.S3.Bucket.Arn)
	assert.Equal(t, TestProjectID.String(), record.S3.Bucket.OwnerIdentity.PrincipalId)
	assert.Equal(t, "object3", record.S3.Object.Key)
	assert.Equal(t, testStreamVersionID(101), record.S3.Object.VersionId)
	assert.Equal(t, int64(37345), record.S3.Object.Size)
	assert.Equal(t, "1861B9003E6CD718", record.S3.Object.Sequencer)
}

func TestConvertModsToEvent_DeleteObjectExactVersion(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/delete-object-exact-version.json")
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
	assert.Equal(t, testStreamVersionID(101), record.S3.Object.VersionId)
	assert.Equal(t, int64(4194304), record.S3.Object.Size)
	assert.Equal(t, "1861B9003E6CD718", record.S3.Object.Sequencer)
}

func TestConvertModsToEvent_DeleteObjectExactVersionUsingObjectLock(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/delete-object-exact-version-using-object-lock.json")
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
	assert.Equal(t, testStreamVersionID(101), record.S3.Object.VersionId)
	assert.Equal(t, int64(4194304), record.S3.Object.Size)
	assert.Equal(t, "1861B9003E6CD718", record.S3.Object.Sequencer)
}

func TestConvertModsToEvent_DeleteObjectLastCommittedPlain(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/delete-object-last-committed-plain.json")
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

func TestConvertModsToEvent_DeleteObjectLastCommittedSuspended(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/delete-object-last-committed-suspended.json")
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
	assert.Equal(t, S3ObjectRemovedDeleteMarkerCreated, record.EventName)
	assert.Equal(t, "1.0", record.S3.S3SchemaVersion)
	assert.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
	assert.Equal(t, TestBucket, record.S3.Bucket.Name)
	assert.Equal(t, "arn:storj:s3:::"+TestBucket, record.S3.Bucket.Arn)
	assert.Equal(t, TestProjectID.String(), record.S3.Bucket.OwnerIdentity.PrincipalId)
	assert.Equal(t, "object1", record.S3.Object.Key)
	assert.Equal(t, testStreamVersionID(101), record.S3.Object.VersionId)
	assert.Zero(t, record.S3.Object.Size)
	assert.Equal(t, "1861B9003E6CD718", record.S3.Object.Sequencer)
}

func TestConvertModsToEvent_DeleteObjectLastCommittedVersioned(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/delete-object-last-committed-versioned.json")
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
	assert.Equal(t, S3ObjectRemovedDeleteMarkerCreated, record.EventName)
	assert.Equal(t, "1.0", record.S3.S3SchemaVersion)
	assert.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
	assert.Equal(t, TestBucket, record.S3.Bucket.Name)
	assert.Equal(t, "arn:storj:s3:::"+TestBucket, record.S3.Bucket.Arn)
	assert.Equal(t, TestProjectID.String(), record.S3.Bucket.OwnerIdentity.PrincipalId)
	assert.Equal(t, "object1", record.S3.Object.Key)
	assert.Equal(t, testStreamVersionID(101), record.S3.Object.VersionId)
	assert.Zero(t, record.S3.Object.Size)
	assert.Equal(t, "1861B9003E6CD718", record.S3.Object.Sequencer)
}

func TestConvertModsToEvent_FinishCopyObject_Delete_Overwrite(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/finish-copy-object-delete-overwrite.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Empty(t, event.Records)
}

func TestConvertModsToEvent_FinishCopyObject_Update(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/finish-copy-object-update.json")
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
	assert.Equal(t, S3ObjectCreatedCopy, record.EventName)
	assert.Equal(t, "1.0", record.S3.S3SchemaVersion)
	assert.Equal(t, "ObjectEvents", record.S3.ConfigurationId)
	assert.Equal(t, TestBucket, record.S3.Bucket.Name)
	assert.Equal(t, "arn:storj:s3:::"+TestBucket, record.S3.Bucket.Arn)
	assert.Equal(t, TestProjectID.String(), record.S3.Bucket.OwnerIdentity.PrincipalId)
	assert.Equal(t, "object1", record.S3.Object.Key)
	assert.Equal(t, testStreamVersionID(101), record.S3.Object.VersionId)
	assert.Equal(t, int64(4194304), record.S3.Object.Size)
	assert.Equal(t, "1861B9003E6CD718", record.S3.Object.Sequencer)
}

func TestConvertModsToEvent_FinishMoveObject_Delete(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/finish-move-object-delete.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Empty(t, event.Records)
}

func TestConvertModsToEvent_FinishMoveObject_Insert(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/finish-move-object-insert.json")
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
	assert.Equal(t, S3ObjectCreatedCopy, record.EventName)
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

func TestConvertModsToEvent_ObjectCopyInsertPending(t *testing.T) {
	var r changestream.DataChangeRecord
	raw, err := os.ReadFile("./testdata/object-copy-insert-pending.json")
	require.NoError(t, err)
	err = json.Unmarshal(raw, &r)
	require.NoError(t, err)
	event, err := ConvertModsToEvent(r)
	require.NoError(t, err)
	require.Empty(t, event.Records)
}

func TestConvertModsToEvent(t *testing.T) {
	testTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("Multiple mods in single record", func(t *testing.T) {
		dataRecord := changestream.DataChangeRecord{
			CommitTimestamp: testTime,
			TransactionTag:  "commit-object",
			ModType:         "INSERT",
			Mods: []*changestream.Mods{
				{
					NewValues: spanner.NullJSON{
						Value: map[string]interface{}{
							"stream_id":        "k3JrjdBKRbuCT2cxhu5vlg==", // base64: TestStreamID
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
							"stream_id":        "pcBVBgOOQmWX34uVMZBi1g==", // base64: TestStreamID
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
			TransactionTag:  "commit-object",
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
			TransactionTag:  "delete-object-last-committed-plain",
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
	for _, tt := range []struct {
		transactionTag    string
		modType           string
		expectedEventName string
	}{
		{"begin-object-exact-version", "INSERT", ""},
		{"begin-object-exact-version", "UPDATE", ""},
		{"begin-object-exact-version", "DELETE", ""},
		{"begin-object-next-version	", "INSERT", ""},
		{"begin-object-next-version	", "UPDATE", ""},
		{"begin-object-next-version	", "DELETE", ""},
		{"commit-inline-object", "INSERT", S3ObjectCreatedPut},
		{"commit-inline-object", "UPDATE", ""},
		{"commit-inline-object", "DELETE", ""},
		{"commit-object", "INSERT", S3ObjectCreatedPut},
		{"commit-object", "UPDATE", S3ObjectCreatedPut},
		{"commit-object", "DELETE", ""},
		{"delete-all-bucket-objects", "INSERT", ""},
		{"delete-all-bucket-objects", "UPDATE", ""},
		{"delete-all-bucket-objects", "DELETE", S3ObjectRemovedDelete},
		{"delete-object-exact-version", "INSERT", ""},
		{"delete-object-exact-version", "UPDATE", ""},
		{"delete-object-exact-version", "DELETE", S3ObjectRemovedDelete},
		{"delete-object-exact-version-using-object-lock", "INSERT", ""},
		{"delete-object-exact-version-using-object-lock", "UPDATE", ""},
		{"delete-object-exact-version-using-object-lock", "DELETE", S3ObjectRemovedDelete},
		{"delete-object-last-committed-plain", "INSERT", ""},
		{"delete-object-last-committed-plain", "UPDATE", ""},
		{"delete-object-last-committed-plain", "DELETE", S3ObjectRemovedDelete},
		{"delete-object-last-committed-suspended", "INSERT", S3ObjectRemovedDeleteMarkerCreated},
		{"delete-object-last-committed-suspended", "UPDATE", ""},
		{"delete-object-last-committed-suspended", "DELETE", ""},
		{"delete-object-last-committed-versioned", "INSERT", S3ObjectRemovedDeleteMarkerCreated},
		{"delete-object-last-committed-versioned", "UPDATE", ""},
		{"delete-object-last-committed-versioned", "DELETE", ""},
		{"finish-copy-object", "INSERT", ""},
		{"finish-copy-object", "UPDATE", S3ObjectCreatedCopy},
		{"finish-copy-object", "DELETE", ""},
		{"finish-move-object", "INSERT", S3ObjectCreatedCopy},
		{"finish-move-object", "UPDATE", ""},
		{"finish-move-object", "DELETE", ""},
		{"object-copy-insert-pending", "INSERT", ""},
		{"object-copy-insert-pending", "UPDATE", ""},
		{"object-copy-insert-pending", "DELETE", ""},
		{"unknown-transaction", "INSERT", ""},
		{"unknown-transaction", "UPDATE", ""},
		{"unknown-transaction", "DELETE", ""},
	} {
		t.Run(fmt.Sprintf("%s %s", tt.transactionTag, tt.modType), func(t *testing.T) {
			eventName := determineEventName(tt.transactionTag, tt.modType)
			assert.Equal(t, tt.expectedEventName, eventName)
		})
	}
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
