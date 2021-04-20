// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/useragent"
)

var (
	// ErrPartners is the default error class for partners package.
	ErrPartners = errs.Class("partners")

	// ErrPartnerNotExist is returned when a particular partner does not exist.
	ErrPartnerNotExist = errs.Class("partner does not exist")
)

// PartnersDB allows access to partners database.
//
// architecture: Database
type PartnersDB interface {
	// All returns all partners.
	All(ctx context.Context) ([]PartnerInfo, error)
	// ByName returns partner definitions for a given name.
	ByName(ctx context.Context, name string) (PartnerInfo, error)
	// ByID returns partner definition corresponding to an id.
	ByID(ctx context.Context, id string) (PartnerInfo, error)
	// ByUserAgent returns partner definition corresponding to an user agent string.
	ByUserAgent(ctx context.Context, agent string) (PartnerInfo, error)
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

// ByName looks up partner by name.
func (service *PartnersService) ByName(ctx context.Context, name string) (PartnerInfo, error) {
	return service.db.ByName(ctx, name)
}

// ByUserAgent looks up partner by user agent.
func (service *PartnersService) ByUserAgent(ctx context.Context, userAgentString string) (PartnerInfo, error) {
	info, err := useragent.Parse(userAgentString)
	if err != nil {
		return PartnerInfo{}, ErrPartners.Wrap(err)
	}

	return service.db.ByUserAgent(ctx, info.Product.Name)
}

// All returns all partners.
func (service *PartnersService) All(ctx context.Context) ([]PartnerInfo, error) {
	return service.db.All(ctx)
}
