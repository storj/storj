// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
)

func TestStreamID_Encode(t *testing.T) {
	_, err := storj.StreamIDFromString("likn43kilfzd")
	assert.Error(t, err)

	_, err = storj.StreamIDFromBytes([]byte{1, 2, 3, 4, 5})
	assert.Error(t, err)

	for i := 0; i < 10; i++ {
		streamID := testrand.StreamID()

		fromString, err := storj.StreamIDFromString(streamID.String())
		assert.NoError(t, err)
		fromBytes, err := storj.StreamIDFromBytes(streamID.Bytes())
		assert.NoError(t, err)

		assert.Equal(t, streamID, fromString)
		assert.Equal(t, streamID, fromBytes)
	}
}
