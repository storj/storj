// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"path"

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
	log     *zap.Logger
	db      PartnersDB
	domains []string
}

// NewPartnersService returns a service for handling partner information.
func NewPartnersService(log *zap.Logger, db PartnersDB, domains []string) *PartnersService {
	return &PartnersService{
		log:     log,
		db:      db,
		domains: domains,
	}
}

// GeneratePartnerLink returns base64 encoded partner referral link.
func (service *PartnersService) GeneratePartnerLink(ctx context.Context, offerName string) ([]string, error) {
	partner, err := service.db.ByName(ctx, offerName)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	referralInfo := &referralInfo{UserID: "", PartnerID: partner.ID}
	refJSON, err := json.Marshal(referralInfo)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	// TODO: why is this using base64?
	encoded := base64.StdEncoding.EncodeToString(refJSON)

	var links []string
	for _, domain := range service.domains {
		links = append(links, path.Join(domain, "ref", encoded))
	}

	return links, nil
}
