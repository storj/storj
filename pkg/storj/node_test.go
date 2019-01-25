// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"

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
		{"0da09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de0f1", 0},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de0fe", 1},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de0fc", 2},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de0f8", 3},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de030", 4},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de0e0", 5},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de0c0", 6},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de080", 7},
		{"fda09d6bed970d7a38fe7389cd2b1b9620cf0ea1fcda2404d353c3fa113de500", 8},
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

		var nodeid storj.NodeID
		n := copy(nodeid[:], decoded)
		if !assert.Equal(t, n, len(nodeid)) {
			t.Fatal()
		}

		difficulty, err := nodeid.Difficulty()
		if !assert.NoError(t, err) {
			t.Fatal()
		}

		assert.Equal(t, testcase.difficulty, difficulty)
	}
}
