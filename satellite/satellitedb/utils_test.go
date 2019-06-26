// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testrand"
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
		id := testrand.UUID()
		result, err := bytesToUUID(id[:])
		assert.NoError(t, err)
		assert.Equal(t, result, id)
	})
}

func TestPostgresNodeIDsArray(t *testing.T) {
	ids := make(storj.NodeIDList, 10)
	for i := range ids {
		ids[i] = testrand.NodeID()
	}

	got, err := postgresNodeIDList(ids).Value() // returns a []byte
	require.NoError(t, err)

	expected, err := pq.ByteaArray(ids.Bytes()).Value() // returns a string
	require.NoError(t, err)

	assert.Equal(t, expected.(string), string(got.([]byte)))
}
