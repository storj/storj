// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/satellite/console"
)

func TestSanitizedOrderColumnName(t *testing.T) {
	testCases := [...]struct {
		orderNumber int8
		orderColumn string
	}{
		0: {0, "u.full_name"},
		1: {1, "u.full_name"},
		2: {2, "u.email"},
		3: {3, "u.created_at"},
		4: {4, "u.full_name"},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.orderColumn, sanitizedOrderColumnName(console.ProjectMemberOrder(tc.orderNumber)))
	}
}
