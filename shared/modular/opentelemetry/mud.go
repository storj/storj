// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package opentelemetry

import (
	"go.opentelemetry.io/otel/sdk/log"

	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module definition.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "otel")
	mud.Provide[*Opentelemetry](ball, NewOpentelemetry)
	mud.View[*Opentelemetry, *log.LoggerProvider](ball, func(opentelemetry *Opentelemetry) *log.LoggerProvider {
		return opentelemetry.Log
	})
}
