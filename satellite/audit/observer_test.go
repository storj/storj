// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/satellite/audit"
)

func TestAuditObserver(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		audits := planet.Satellites[0].Audit.Service
		satellite := planet.Satellites[0]
		err := audits.Close()
		require.NoError(t, err)

		observer := audit.NewObserver(zaptest.NewLogger(t), satellite.Overlay.Service, audit.ReservoirConfig{1, 1})
		err = audits.MetainfoLoop.Join(ctx, observer)
		require.NoError(t, err)

		// todo: make sure RemoteSegment function is creating reservoirs when it should
	})
}
