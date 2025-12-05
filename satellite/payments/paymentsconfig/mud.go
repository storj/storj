// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig

import (
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
}
