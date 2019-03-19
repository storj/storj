// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestUploadDownload(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 10, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	expectedData := make([]byte, 5*memory.MiB)
	_, err = rand.Read(expectedData)
	assert.NoError(t, err)

	err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test/bucket", "test/path", expectedData)
	assert.NoError(t, err)

	data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "test/bucket", "test/path")
	assert.NoError(t, err)

	assert.Equal(t, expectedData, data)
}
