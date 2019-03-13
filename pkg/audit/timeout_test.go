// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/audit"
)

func TestAuditTimeout(t *testing.T) {
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

	pointers := planet.Satellites[0].Metainfo.Service
	allocation := planet.Satellites[0].Metainfo.Allocation
	cursor := audit.NewCursor(pointers, allocation, planet.Satellites[0].Identity)

	stripe, err := cursor.NextStripe(ctx)
	if err != nil {
		assert.Error(t, err)
		assert.Nil(t, stripe)
	}

	overlay := planet.Satellites[0].Overlay.Service
	transport := planet.Satellites[0].Transport

	verifier := audit.NewVerifier(transport, overlay, planet.Satellites[0].Identity)

	_, err = verifier.Verify(ctx, stripe)
	assert.Error(t, err)
}
