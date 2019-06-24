// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"crypto/rand"
	"testing"

	"github.com/lib/pq"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/storj"
)

func TestBytesToUUID(t *testing.T) {
	t.Run("Invalid input", func(t *testing.T) {
		str := "not UUID string"
		bytes := []byte(str)

		_, err := bytesToUUID(bytes)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Valid input", func(t *testing.T) {
		id, err := uuid.New()
		assert.NoError(t, err)

		result, err := bytesToUUID(id[:])
		assert.NoError(t, err)
		assert.Equal(t, result, *id)
	})
}

func TestSpliteBucketID(t *testing.T) {
	t.Run("Invalid input", func(t *testing.T) {
		str := "not UUID string/bucket1"
		bytes := []byte(str)

		_, _, err := splitBucketID(bytes)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Valid input", func(t *testing.T) {
		expectedBucketID, err := uuid.Parse("bb6218e3-4b4a-4819-abbb-fa68538e33c0")
		expectedBucketName := "bucket1"
		assert.NoError(t, err)

		str := expectedBucketID.String() + "/" + expectedBucketName

		bucketID, bucketName, err := splitBucketID([]byte(str))

		assert.NoError(t, err)
		assert.Equal(t, bucketID, expectedBucketID)
		assert.Equal(t, bucketName, []byte(expectedBucketName))
	})
}

func TestPostgresNodeIDsArray(t *testing.T) {
	ids := make(storj.NodeIDList, 10)
	for i := range ids {
		_, _ = rand.Read(ids[i][:])
	}

	got, err := postgresNodeIDList(ids).Value() // returns a []byte
	require.NoError(t, err)

	expected, err := pq.ByteaArray(ids.Bytes()).Value() // returns a string
	require.NoError(t, err)

	assert.Equal(t, expected.(string), string(got.([]byte)))
}
