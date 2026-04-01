// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"go.uber.org/zap"

	"storj.io/storj/satellite/admin/auditlogger"
	"storj.io/storj/satellite/admin/changehistory"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "admin")

	mud.Provide[*Authorizer](ball, NewAuthorizer)

	mud.Provide[*auditlogger.Logger](ball, func(log *zap.Logger, analyticsService *analytics.Service, history changehistory.DB, cfg Config) *auditlogger.Logger {
		return auditlogger.New(log.Named("audit-logger"), analyticsService, history, cfg.ExternalAddress, cfg.AuditLogger)
	})

	mud.Provide[*Service](ball, NewService)
}
