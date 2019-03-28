// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcutil/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/storj"
)

func TestNodeID_Difficulty(t *testing.T) {
	invalidID := storj.NodeID{}
	difficulty, err := invalidID.Difficulty()
	assert.Error(t, err)
	assert.Equal(t, uint16(0), difficulty)

	for _, testcase := range []struct {
		id         string
		difficulty uint16
	}{
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113dee00", 9},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113dec00", 10},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de800", 11},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113d7000", 12},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de000", 13},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113dc000", 14},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113d8000", 15},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa11390000", 16},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113e0000", 17},
	} {

		decoded, err := hex.DecodeString(testcase.id)
		if !assert.NoError(t, err) {
			t.Fatal()
		}

		var nodeID storj.NodeID
		n := copy(nodeID[:], decoded)
		if !assert.Equal(t, n, len(nodeID)) {
			t.Fatal()
		}

		difficulty, err := nodeID.Difficulty()
		if !assert.NoError(t, err) {
			t.Fatal()
		}

		assert.Equal(t, testcase.difficulty, difficulty)
	}
}

// TestNodeScan tests (*NodeID).Scan()
func TestNodeScan(t *testing.T) {
	tmpID := &storj.NodeID{}
	require.Error(t, tmpID.Scan(32))
	require.Error(t, tmpID.Scan(false))
	require.Error(t, tmpID.Scan([]byte{}))
	require.NoError(t, tmpID.Scan(tmpID.Bytes()))
}

// TestNodeValue tests NodeID.Value()
func TestNodeValue(t *testing.T) {
	tmpID := storj.NodeID{}
	v, err := tmpID.Value()
	require.NoError(t, err)
	require.IsType(t, v, []byte{})
	require.Len(t, v, storj.NodeIDSize)
}

func TestNodeID_Version(t *testing.T) {
	for _, testcase := range []struct {
		id         string
		difficulty uint16
		version    storj.IDVersionNumber
	}{
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de500", 8, storj.V1},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113dee00", 9, storj.V1},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113dec00", 10, storj.V1},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de502", 8, storj.V2},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113dee02", 9, storj.V2},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113dec02", 10, storj.V2},
	} {
		decoded, err := hex.DecodeString(testcase.id)
		require.NoError(t, err)

		var nodeID storj.NodeID
		n := copy(nodeID[:], decoded)
		require.Equal(t, n, len(nodeID))

		difficulty, err := nodeID.Difficulty()
		require.NoError(t, err)

		assert.Equal(t, testcase.difficulty, difficulty)
		assert.Equal(t, testcase.version, nodeID.Version().Number)
	}
}

func TestNodeID_String_Version(t *testing.T) {
	for _, testcase := range []struct {
		hexID    string
		base58ID string
		version  storj.IDVersionNumber
	}{
		{
			"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de500",
			"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dBZN6Lg2T",
			storj.V1,
		},
		{
			"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113dee00",
			"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dG3JN2sdZ",
			storj.V1,
		},
		{
			"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113dec00",
			"12vha9oTFnerxYRgeQ2BZqoFrLrnmmf5UWTCY2jA77dF3YvWew7",
			storj.V1,
		},
		{
			"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de502",
			"6pBvchWxe1KEBb6TZGBUUzoga6E5BLq3nbR2L6x6RdxYeH2Adm",
			storj.V2,
		},
		{
			"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113dee02",
			"6pBvchWxe1KEBb6TZGBUUzoga6E5BLq3nbR2L6x6Re32bKJBBk",
			storj.V2,
		},
		{
			"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113dec02",
			"6pBvchWxe1KEBb6TZGBUUzoga6E5BLq3nbR2L6x6Re22qchU4f",
			storj.V2,
		},
	} {
		decoded, err := hex.DecodeString(testcase.hexID)
		require.NoError(t, err)

		var nodeID storj.NodeID
		n := copy(nodeID[:], decoded)
		require.Equal(t, n, len(nodeID))

		base58Str := nodeID.String()
		binID, version, err := base58.CheckDecode(base58Str)
		require.NoError(t, err)

		idVersion, err := storj.GetIDVersion(storj.IDVersionNumber(version))
		require.NoError(t, err)

		assert.Equal(t, testcase.version, idVersion.Number)
		assert.Equal(t, nodeID[:storj.NodeIDSize-1], binID[:storj.NodeIDSize-1])
	}
}

func TestNewVersionedID(t *testing.T) {
	nodeID := storj.NodeID{}
	_, err := rand.Read(nodeID[:])
	require.NoError(t, err)
	nodeID[storj.NodeIDSize-1] = 0

	assert.Equal(t, storj.V1, nodeID.Version().Number)

	for versionNumber, version := range storj.IDVersions {
		versionedNodeID := storj.NewVersionedID(nodeID, version)
		assert.Equal(t, versionNumber, versionedNodeID.Version().Number)
		assert.Equal(t, versionNumber, storj.IDVersionNumber(versionedNodeID[storj.NodeIDSize-1]))
	}
}
