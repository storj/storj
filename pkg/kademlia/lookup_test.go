// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestLookupNodes(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	k := planet.Satellites[0].Kademlia.Service
	k.WaitForBootstrap() // redundant, but leaving here to be clear

	seen := k.Seen()
	assert.NotEqual(t, len(seen), 0)
	assert.NotNil(t, seen)

	target := seen[0]
	found, err := k.FindNode(ctx, target.Id)
	assert.NoError(t, err)
	assert.Equal(t, target.Id, found.Id)
}
