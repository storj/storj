// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestUploadDownload(t *testing.T) {
	tctx := testcontext.New(t)
	defer tctx.Cleanup()

	planet, err := testplanet.New(t, 1, 10, 1)
	require.NoError(t, err)
	defer tctx.Check(planet.Shutdown)

	planet.Start(tctx)
	time.Sleep(2 * time.Second)

	expectedData := make([]byte, 1024*1024*5)
	rand.Read(expectedData)

	err = planet.Uplinks[0].Upload(tctx, planet.Satellites[0], "test/bucket", "test/path", expectedData)
	assert.NoError(t, err)

	data, err := planet.Uplinks[0].Download(tctx, planet.Satellites[0], "test/bucket", "test/path")
	assert.NoError(t, err)

	assert.Equal(t, true, bytes.Equal(expectedData, data))
}
