// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package entitlements

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// Error is the base error class for the entitlements service.
var Error = errs.Class("entitlements service")

// Service represents the entitlements service.
type Service struct {
	log *zap.Logger
	db  DB
}

// NewService creates a new entitlements service.
func NewService(log *zap.Logger, db DB) *Service {
	return &Service{
		log: log,
		db:  db,
	}
}

// Projects returns all projects related functionality.
func (s *Service) Projects() *Projects {
	return &Projects{service: s}
}
