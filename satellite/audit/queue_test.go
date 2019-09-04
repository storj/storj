// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/storj"
)

func TestQueue(t *testing.T) {
	q := &Queue{
		queue: []storj.Path{},
	}
	_, err := q.Next()
	require.True(t, ErrEmptyQueue.Has(err), "required ErrEmptyQueue error")

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

	path, err = q.Next()
	require.NoError(t, err)
	require.EqualValues(t, testQueue2[0], path)

	path, err = q.Next()
	require.NoError(t, err)
	require.EqualValues(t, testQueue2[1], path)

	path, err = q.Next()
	require.NoError(t, err)
	require.EqualValues(t, testQueue2[2], path)

	_, err = q.Next()
	require.True(t, ErrEmptyQueue.Has(err), "required ErrEmptyQueue error")
}
