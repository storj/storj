// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateArgumentsDBX(t *testing.T) {
	testCases := [...]struct {
		length         int
		expectedResult string
	}{
		0: {0, "()"},
		1: {1, "(?)"},
		2: {2, "(?, ?)"},
		3: {3, "(?, ?, ?)"},
		4: {-1, "()"},
		5: {-2, "()"},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedResult, generateArgumentsDBX(tc.length))
	}
}
