// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

var (
	// Error is the default error class for partners package.
	Error = errs.Class("partners error class")

	// ErrNotExist is returned when a particular partner does not exist.
	ErrNotExist = errs.Class("partner does not exist")
)

// PartnersDB allows access to partners database.
//
// architecture: Database
type PartnersDB interface {
	// All returns all partners.
	All(ctx context.Context) ([]Partner, error)
	// ByName returns partner definitions for a given name.
	ByName(ctx context.Context, name string) (Partner, error)
	// ByID returns partner definition corresponding to an id.
	ByID(ctx context.Context, id string) (Partner, error)
	// ByUserAgent returns partner definition corresponding to an user agent string.
	ByUserAgent(ctx context.Context, agent string) (Partner, error)
}

// PartnersService allows manipulating and accessing partner information.
//
// architecture: Service
type PartnersService struct {
	log *zap.Logger
	db  PartnersDB
}

// NewPartnersService returns a service for handling partner information.
func NewPartnersService(log *zap.Logger, db PartnersDB) *PartnersService {
	return &PartnersService{
		log: log,
		db:  db,
	}
}
