// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"go.uber.org/zap"

	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/nodeselection"
)

// Defaults contains default values for limits which are not stored in the DB.
type Defaults struct {
	MaxBuckets int
	RateLimit  int
}

// Service provides functionality for administrating satellites.
type Service struct {
	log          *zap.Logger
	consoleDB    console.DB
	accountingDB accounting.ProjectAccounting
	accounting   *accounting.Service
	placement    nodeselection.PlacementDefinitions
	defaults     Defaults
}

// NewService creates a new satellite administration service.
func NewService(
	log *zap.Logger,
	consoleDB console.DB,
	accountingDB accounting.ProjectAccounting,
	accounting *accounting.Service,
	placement nodeselection.PlacementDefinitions,
	defaultMaxBuckets int,
	defaultRateLimit float64,
) *Service {
	return &Service{
		log:          log,
		consoleDB:    consoleDB,
		accountingDB: accountingDB,
		accounting:   accounting,
		placement:    placement,
		defaults: Defaults{
			MaxBuckets: defaultMaxBuckets,
			RateLimit:  int(defaultRateLimit),
		},
	}
}
