// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2_test

import (
	"context"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/internal/testrand"
)

func TestCopy(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := io.LimitReader(testrand.Reader(), 32*memory.KiB.Int64())

	n, err := sync2.Copy(ctx, ioutil.Discard, r)

	assert.NoError(t, err)
	assert.Equal(t, n, 32*memory.KiB.Int64())
}

func TestCopy_Cancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := io.LimitReader(testrand.Reader(), 32*memory.KiB.Int64())

	n, err := sync2.Copy(ctx, ioutil.Discard, r)

	assert.EqualError(t, err, context.Canceled.Error())
	assert.EqualValues(t, n, 0)
}
