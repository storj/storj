// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/storj/satellite/audit"
)

func TestQueue(t *testing.T) {
	q := &audit.Queue{}

	_, err := q.Next()
	require.True(t, audit.ErrEmptyQueue.Has(err), "required ErrEmptyQueue error")

	testQueue1 := []storj.Path{"a", "b", "c"}
	q.Swap(testQueue1)
	path, err := q.Next()
	require.NoError(t, err)
	require.EqualValues(t, testQueue1[0], path)

	path, err = q.Next()
	require.NoError(t, err)
	require.EqualValues(t, testQueue1[1], path)

	testQueue2 := []storj.Path{"0", "1", "2"}
	q.Swap(testQueue2)

	for _, expected := range testQueue2 {
		path, err := q.Next()
		require.NoError(t, err)
		require.EqualValues(t, expected, path)
	}

	_, err = q.Next()
	require.True(t, audit.ErrEmptyQueue.Has(err), "required ErrEmptyQueue error")
}
