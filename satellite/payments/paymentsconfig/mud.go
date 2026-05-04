// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig

import (
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module definition.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "payments")
	mud.View[Config, stripe.Config](ball, func(c Config) stripe.Config {
		return c.StripeCoinPayments
	})
	mud.Provide[map[int32]payments.ProductUsagePriceModel](ball, func(cfg Config) (map[int32]payments.ProductUsagePriceModel, error) {
		return cfg.Products.ToModels()
	})
	mud.Provide[payments.PlacementProductIdMap](ball, func(cfg Config) payments.PlacementProductIdMap {
		return cfg.PlacementPriceOverrides.ToMap()
	})
	// TODO: move billing.Config out from payments.Config
	mud.Provide[billing.Config](ball, func(cfg Config) billing.Config {
		return cfg.BillingConfig
	})
}
