// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/storj"
)

func TestQueue(t *testing.T) {
	queue := &Queue{
		Queue: []storj.Path{},
	}
	_, err := queue.Next()
	require.True(t, ErrEmptyQueue.Has(err))

	testQueue1 := []storj.Path{"a", "b", "c"}
	queue.Swap(testQueue1)
	path, err := queue.Next()
	require.NoError(t, err)
	require.EqualValues(t, testQueue1[0], path)

	path, err = queue.Next()
	require.NoError(t, err)
	require.EqualValues(t, testQueue1[1], path)

	testQueue2 := []storj.Path{"0", "1", "2"}
	queue.Swap(testQueue2)

	path, err = queue.Next()
	require.NoError(t, err)
	require.EqualValues(t, testQueue2[0], path)

	path, err = queue.Next()
	require.NoError(t, err)
	require.EqualValues(t, testQueue2[1], path)

	path, err = queue.Next()
	require.NoError(t, err)
	require.EqualValues(t, testQueue2[2], path)

	path, err = queue.Next()
	require.True(t, ErrEmptyQueue.Has(err))
}
