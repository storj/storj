// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/emission"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/shared/mud"
)

// Module is a mud module definition.
func Module(ball *mud.Ball) {
	mud.Provide[*Service](ball, NewService)
	mud.Provide[ServiceDependencies](ball, func(db DB, walletsDB storjscan.WalletsDB, billingDB billing.TransactionsDB, projectsDB console.Projects, usersDB console.Users,
		usageDB accounting.ProjectAccounting, analytics *analytics.Service, emission *emission.Service, entitlements *entitlements.Service) ServiceDependencies {
		return ServiceDependencies{
			DB:           db,
			WalletsDB:    walletsDB,
			BillingDB:    billingDB,
			ProjectsDB:   projectsDB,
			UsersDB:      usersDB,
			UsageDB:      usageDB,
			Analytics:    analytics,
			Emission:     emission,
			Entitlements: entitlements,
		}
	})
	mud.Provide[Client](ball, NewStripeClient)
	mud.Provide[payments.Accounts](ball, func(s *Service) payments.Accounts {
		return s.Accounts()
	})
}
