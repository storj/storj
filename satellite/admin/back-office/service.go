// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"time"

	"go.uber.org/zap"

	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/admin/back-office/auditlogger"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/payments"
)

// Defaults contains default values for limits which are not stored in the DB.
type Defaults struct {
	MaxBuckets int
	RateLimit  int
}

// Service provides functionality for administrating satellites.
type Service struct {
	log *zap.Logger

	authorizer  *Authorizer
	auditLogger *auditlogger.Logger

	accountingDB accounting.ProjectAccounting
	consoleDB    console.DB
	restKeys     restapikeys.Service

	accountFreeze *console.AccountFreezeService
	accounting    *accounting.Service
	analytics     *analytics.Service
	payments      payments.Accounts

	placement nodeselection.PlacementDefinitions
	defaults  Defaults

	consoleConfig console.Config

	nowFn func() time.Time
}

// NewService creates a new satellite administration service.
func NewService(
	log *zap.Logger,
	consoleDB console.DB,
	accountingDB accounting.ProjectAccounting,
	accounting *accounting.Service,
	authorizer *Authorizer,
	accountFreeze *console.AccountFreezeService,
	analytics *analytics.Service,
	payments payments.Accounts,
	restKeys restapikeys.Service,
	placement nodeselection.PlacementDefinitions,
	defaultMaxBuckets int,
	defaultRateLimit float64,
	auditLoggerConfig auditlogger.Config,
	consoleConfig console.Config,
	nowFn func() time.Time,
) *Service {
	return &Service{
		log:           log,
		consoleDB:     consoleDB,
		restKeys:      restKeys,
		analytics:     analytics,
		accountingDB:  accountingDB,
		accounting:    accounting,
		accountFreeze: accountFreeze,
		authorizer:    authorizer,
		auditLogger:   auditlogger.New(log.Named("audit-logger"), analytics, auditLoggerConfig),
		payments:      payments,
		placement:     placement,
		defaults: Defaults{
			MaxBuckets: defaultMaxBuckets,
			RateLimit:  int(defaultRateLimit),
		},
		consoleConfig: consoleConfig,
		nowFn:         nowFn,
	}
}

// TestSetBypassAuth sets whether to bypass authentication. This is only for testing purposes.
func (s *Service) TestSetBypassAuth(bypass bool) {
	s.authorizer.enabled = !bypass
}

// TestSetAllowedHost sets the allowed host for oauth. This is only for testing purposes.
func (s *Service) TestSetAllowedHost(host string) {
	s.authorizer.allowedHost = host
}

// TestSetNowFn sets the function to get the current time. This is only for testing purposes.
func (s *Service) TestSetNowFn(nowFn func() time.Time) {
	s.nowFn = nowFn
}
