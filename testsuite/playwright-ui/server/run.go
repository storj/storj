// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest

import (
	"os"
	"testing"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
)

var mon = monkit.Package()

// Test defines common services for uitests.
type Test func(t *testing.T, ctx *testcontext.Context, planet *EdgePlanet)

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
	config.Console.SignupActivationCodeEnabled = false
	config.Console.CouponCodeBillingUIEnabled = true
	config.Console.RateLimit.Burst = 10000
	config.SeparateConsoleAPI = false
}

// Run starts a new UI test.
func Run(t *testing.T, test Test) {
	Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *EdgePlanet) {
		test(t, ctx, planet)
	})
}
