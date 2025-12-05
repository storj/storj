// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

type pauseQueueingFunc = func(satellite *testplanet.Satellite)
type runQueueingOnceFunc = func(ctx context.Context, satellite *testplanet.Satellite) error

// testWithRangedLoop runs an audit test for both the chore and observer.
// It provides functions that the test can use to pause and run the queueing
// done by the chore or observer.
func testWithRangedLoop(t *testing.T, planetConfig testplanet.Config, run func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc)) {
	t.Run("Observer", func(t *testing.T) {
		planetConfig := planetConfig
		reconfigureSatellite := planetConfig.Reconfigure.Satellite
		planetConfig.Reconfigure.Satellite = func(log *zap.Logger, index int, config *satellite.Config) {
			if reconfigureSatellite != nil {
				reconfigureSatellite(log, index, config)
			}
			config.Audit.UseRangedLoop = true
		}
		testplanet.Run(t, planetConfig, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			t.Helper()
			run(t, ctx, planet,
				func(satellite *testplanet.Satellite) {},
				func(ctx context.Context, satellite *testplanet.Satellite) error {
					_, err := satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
					return err
				},
			)
		})
	})
}
