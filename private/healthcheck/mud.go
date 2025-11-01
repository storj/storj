// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package healthcheck

import (
	"net"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module definition.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "healthcheck")
	mud.RegisterImplementation[[]HealthCheck](ball)
	mud.Provide[*Server](ball, func(log *zap.Logger, cfg Config, checks []HealthCheck) (*Server, error) {

		listener, err := net.Listen("tcp", cfg.Address)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		return NewServer(log, listener, checks...), nil
	})
}
