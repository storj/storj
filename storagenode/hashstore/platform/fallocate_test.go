// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package platform

import (
	"os"
	"testing"

	"github.com/zeebo/assert"
)

func TestFallocateKeepsSize(t *testing.T) {
	fh, err := os.CreateTemp(t.TempDir(), "fallocate-test-*")
	assert.NoError(t, err)
	defer func() { _ = fh.Close() }()

	_, err = fh.WriteString("hello")
	assert.NoError(t, err)

	assert.NoError(t, Fallocate(fh, 4096))

	fi, err := fh.Stat()
	assert.NoError(t, err)
	assert.Equal(t, fi.Size(), int64(5))
}
