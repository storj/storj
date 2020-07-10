// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

// TestPaywallEnabled ensures that the Paywall A/B test config works.
func TestPaywallEnabled(t *testing.T) {
	lowUUID := uuid.UUID{0}
	highUUID := uuid.UUID{255}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 3, Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				proportions := []float64{0, .5, 1}
				config.Payments.PaywallProportion = proportions[index]
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		assert.False(t, planet.Satellites[0].API.Console.Service.PaywallEnabled(highUUID))
		assert.False(t, planet.Satellites[0].API.Console.Service.PaywallEnabled(lowUUID))
		assert.False(t, planet.Satellites[1].API.Console.Service.PaywallEnabled(highUUID))
		assert.True(t, planet.Satellites[1].API.Console.Service.PaywallEnabled(lowUUID))
		assert.True(t, planet.Satellites[2].API.Console.Service.PaywallEnabled(highUUID))
		assert.True(t, planet.Satellites[2].API.Console.Service.PaywallEnabled(lowUUID))
	})
}
