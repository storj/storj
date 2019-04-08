// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestGetSignee(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		trust := planet.StorageNodes[0].Storage2.Trust

		canceledContext, cancel := context.WithCancel(ctx)
		cancel()

		// GetSignee should succeed even on canceled context to avoid miscaching
		cert, err := trust.GetSignee(canceledContext, planet.Satellites[0].ID())
		assert.NoError(t, err)
		assert.NotNil(t, cert)
	})
}
