// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
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

		assert.Nil(t, err)
		assert.NoError(t, err)

		assert.Equal(t, result, *id)
	})
}
