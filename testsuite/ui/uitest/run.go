// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest

import (
	"os"
	"testing"

	"github.com/go-rod/rod"
	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

var mon = monkit.Package()

// Test defines common services for uitests.
type Test func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser)

type zapWriter struct {
	*zap.Logger
}

func (log zapWriter) Write(data []byte) (int, error) {
	log.Logger.Info(string(data))
	return len(data), nil
}

func configureSatellite(log *zap.Logger, index int, config *satellite.Config) {
	if dir := os.Getenv("STORJ_TEST_SATELLITE_WEB"); dir != "" {
		config.Console.StaticDir = dir
	}
	config.Console.CouponCodeBillingUIEnabled = true
}

// Run starts a new UI test.
func Run(t *testing.T, test Test) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: configureSatellite,
		},
		NonParallel: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		Browser(t, ctx, planet, func(browser *rod.Browser) {
			test(t, ctx, planet, browser)
		})
	})
}
