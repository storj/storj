// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"net"

	"go.uber.org/zap"

	"storj.io/common/version"
	"storj.io/storj/private/server"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/shared/mud"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/operator"
	"storj.io/storj/storagenode/payouts/estimatedpayouts"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
)

// Module registers the console service dependency injection components.
func Module(ball *mud.Ball) {
	mud.Provide[*Service](ball, func(log *zap.Logger, bandwidth bandwidth.DB, version *checker.Service,
		versionInfo version.Info, trust *trust.Pool,
		reputationDB reputation.DB, storageUsageDB storageusage.DB, pricingDB pricing.DB, satelliteDB satellites.DB,
		pingStats *contact.PingStats, contact *contact.Service, estimation *estimatedpayouts.Service,
		walletFeatures operator.WalletFeatures, quicStats *contact.QUICStats,
		spaceReport monitor.SpaceReport, server *server.Server, config operator.Config) (*Service, error) {

		_, port, _ := net.SplitHostPort(server.Addr().String())
		return NewService(log, bandwidth, version,
			config.Wallet, versionInfo, trust,
			reputationDB, storageUsageDB, pricingDB, satelliteDB,
			pingStats, contact, estimation,
			config.WalletFeatures, port, quicStats,
			spaceReport)
	})
	mud.View[operator.Config, operator.WalletFeatures](ball, func(config operator.Config) operator.WalletFeatures {
		return config.WalletFeatures
	})
}
