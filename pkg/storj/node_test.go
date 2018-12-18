// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/storj"
)

func TestNodeID_Difficulty(t *testing.T) {
	invalidID := storj.NodeID{}
	difficulty, err := invalidID.Difficulty()
	assert.Error(t, err)
	assert.Equal(t, uint16(0), difficulty)

	// node id with difficulty 13
	node13 := storj.NodeID{253, 160, 157, 107, 237, 151, 13, 122, 56, 254, 115, 137, 205, 43, 27, 150, 32, 207, 14, 161, 252, 218, 36, 4, 211, 83, 195, 250, 17, 61, 224, 0}
	difficulty, err = node13.Difficulty()
	assert.NoError(t, err)
	assert.Equal(t, uint16(13), difficulty)
}
