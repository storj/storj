// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testrand"
)

func TestBytesToUUID(t *testing.T) {
	t.Run("Invalid input", func(t *testing.T) {
		str := "not UUID string"
		bytes := []byte(str)

		_, err := BytesToUUID(bytes)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Valid input", func(t *testing.T) {
		id := testrand.UUID()
		result, err := BytesToUUID(id[:])
		assert.NoError(t, err)
		assert.Equal(t, result, id)
	})
}
