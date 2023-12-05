// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"go.uber.org/zap"

	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/nodeselection"
)

// Service provides functionality for administrating satellites.
type Service struct {
	log          *zap.Logger
	consoleDB    console.DB
	accountingDB accounting.ProjectAccounting
	accounting   *accounting.Service
	placement    nodeselection.PlacementDefinitions
}

// NewService creates a new satellite administration service.
func NewService(
	log *zap.Logger,
	consoleDB console.DB,
	accountingDB accounting.ProjectAccounting,
	accounting *accounting.Service,
	placement nodeselection.PlacementDefinitions,
) *Service {
	return &Service{
		log:          log,
		consoleDB:    consoleDB,
		accountingDB: accountingDB,
		accounting:   accounting,
		placement:    placement,
	}
}
