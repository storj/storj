// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest

import (
	"os"
	"testing"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/kms"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/payments/paymentsconfig"
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
	config.DisableConsoleFromSatelliteAPI = false

	configureSelfServePlacement(config)
	configureWhiteLabel(config)
}

func configureWhiteLabel(config *satellite.Config) {
	config.Console.WhiteLabel.Value = map[string]console.WhiteLabelConfig{
		"tenant1": {
			TenantID:   "tenant1",
			HostName:   "tenant1.localhost.test",
			Name:       "Tenant One",
			SupportURL: "https://support.tenant1.example",
		},
		"tenant2": {
			TenantID:   "tenant2",
			HostName:   "tenant2.localhost.test",
			Name:       "Tenant Two",
			SupportURL: "https://support.tenant2.example",
		},
	}
	config.Console.WhiteLabel.HostNameIDLookup = map[string]string{
		"tenant1.localhost.test": "tenant1",
		"tenant2.localhost.test": "tenant2",
	}
}

func configureSelfServePlacement(config *satellite.Config) {
	placement0 := storj.DefaultPlacement
	placement1 := storj.PlacementConstraint(3)
	placementDetail0 := console.PlacementDetail{
		ID:          0,
		IdName:      "global",
		Name:        "Global",
		Title:       "Globally Distributed",
		Description: "The data is globally distributed.",
	}
	placementDetail1 := console.PlacementDetail{
		ID:          3,
		IdName:      "us-select-1",
		Name:        "Storj Select",
		Title:       "Storj US Select",
		Description: "Store data only on Select nodes in the United States.",
	}
	productID0 := int32(1)
	productID1 := int32(2)
	productPrice0 := paymentsconfig.ProductUsagePrice{
		Name: "Global",
		ProjectUsagePrice: paymentsconfig.ProjectUsagePrice{
			StorageTB: "4",
			EgressTB:  "7",
			Segment:   "0.0000088",
		},
	}
	productPrice1 := paymentsconfig.ProductUsagePrice{
		Name: "Select",
		ProjectUsagePrice: paymentsconfig.ProjectUsagePrice{
			StorageTB: "8",
			EgressTB:  "10",
			Segment:   "0.0000088",
		},
	}
	config.Payments.Products.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
		productID0: productPrice0,
		productID1: productPrice1,
	})
	config.Payments.PlacementPriceOverrides.SetMap(map[int]int32{
		int(placement0): productID0,
		int(placement1): productID1,
	})
	config.Console.Placement.SelfServeEnabled = true
	config.Console.EnableRegionTag = true
	config.Placement = nodeselection.ConfigurablePlacementRule{
		PlacementRules: `3:annotation("location","us-select-1");0:annotation("location","global")`,
	}
	config.Console.Placement.SelfServeDetails.SetMap(map[storj.PlacementConstraint]console.PlacementDetail{
		placement0: placementDetail0,
		placement1: placementDetail1,
	})
	config.Console.SatelliteManagedEncryptionEnabled = true
	config.KeyManagement.MockClient = true
	config.KeyManagement.KeyInfos = kms.KeyInfos{
		Values: map[int]kms.KeyInfo{
			1: {
				SecretVersion: "secretversion1", SecretChecksum: 12345,
			},
		},
	}
}

// Run starts a new UI test.
func Run(t *testing.T, test Test) {
	Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *EdgePlanet) {
		test(t, ctx, planet)
	})
}
