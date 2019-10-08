// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package partners

import (
	"go.uber.org/zap"
)

// Service allows manipulating and accessing partner information.
type Service struct {
	log *zap.Logger
	db  DB
}

// NewService returns a service for handling partner information.
func NewService(log *zap.Logger, db DB) *Service {
	return &Service{
		log: log,
		db:  db,
	}
}
