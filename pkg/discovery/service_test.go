// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package discovery_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestCache_Refresh(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 30, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	err = planet.Satellites[0].Discovery.Service.Refresh(ctx)
	assert.NoError(t, err)
}
