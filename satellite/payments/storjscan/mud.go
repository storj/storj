// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan

import (
	"storj.io/storj/satellite/payments"
	"storj.io/storj/shared/mud"
)

// Module is a mud module definition.
func Module(ball *mud.Ball) { /**/
	mud.Provide[*Client](ball, func(cfg Config) *Client {
		return NewClient(cfg.Endpoint, cfg.Auth.Identifier, cfg.Auth.Secret)
	})
	mud.View[*Service, payments.DepositWallets](ball, func(service *Service) payments.DepositWallets {
		return service
	})
}
