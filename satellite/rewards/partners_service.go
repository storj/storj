// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import (
	"context"
	"encoding/base32"
	"path"

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

// parnterIDEncoding is base32 without padding
var parnterIDEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// GeneratePartnerLink returns partner referral link.
func (service *PartnersService) GeneratePartnerLink(ctx context.Context, offerName string) ([]string, error) {
	partner, err := service.db.ByName(ctx, offerName)
	if err != nil {
		return nil, ErrPartners.Wrap(err)
	}

	var links []string
	for _, domain := range service.domains {
		encoded := parnterIDEncoding.EncodeToString([]byte(partner.ID))
		links = append(links, path.Join(domain, "ref", encoded))
	}

	return links, nil
}

// GetActiveOffer returns an offer that is active based on its type.
func (service *PartnersService) GetActiveOffer(ctx context.Context, offers Offers, offerType OfferType, partnerID string) (offer *Offer, err error) {
	if len(offers) < 1 {
		return nil, ErrOfferNotExist.New("no active offers")
	}
	switch offerType {
	case Partner:
		if partnerID == "" {
			return nil, errs.New("partner ID is empty")
		}
		partnerInfo, err := service.db.ByID(ctx, partnerID)
		if err != nil {
			return nil, ErrPartnerNotExist.Wrap(err)
		}
		for i := range offers {
			if offers[i].Name == partnerInfo.Name {
				offer = &offers[i]
			}
		}
	default:
		if len(offers) > 1 {
			return nil, errs.New("multiple active offers found")
		}
		offer = &offers[0]
	}

	return offer, nil
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

	return service.db.ByName(ctx, info.Product.Name)
}

// All returns all partners.
func (service *PartnersService) All(ctx context.Context) ([]PartnerInfo, error) {
	return service.db.All(ctx)
}
