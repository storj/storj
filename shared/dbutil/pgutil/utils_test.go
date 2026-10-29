// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testrand"
)

func TestPostgresNodeIDsArray(t *testing.T) {
	ids := make(storj.NodeIDList, 10)
	for i := range ids {
		ids[i] = testrand.NodeID()
	}

	got, err := EncodeArray(NodeIDArray(ids))
	require.NoError(t, err)

	expected, err := EncodeArray(ByteaArray(ids.Bytes()))
	require.NoError(t, err)

	assert.Equal(t, expected, got)

	var decoded [][]byte
	require.NoError(t, ScanArray(got, &decoded))
	assert.Equal(t, ids.Bytes(), decoded)
}

func TestScanArrayNull(t *testing.T) {
	decoded := []string{"leftover"}
	require.NoError(t, ScanArray(nil, &decoded))
	assert.Nil(t, decoded)
}
