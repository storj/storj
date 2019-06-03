// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package marketing

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	// Error the default offers errs class
	Error = errs.Class("marketing error")

	mon = monkit.Package()
)

// Service allows access to offers info in the db
type Service struct {
	log *zap.Logger
	db  DB
}

// NewService creates a new offers db
func NewService(log *zap.Logger, db DB) (*Service, error) {
	if log == nil {
		return nil, Error.New("log can't be nil")
	}

	return &Service{
		log: log,
		db:  db,
	}, nil
}

// ListAllOffers returns all available offers in the db
func (s *Service) ListAllOffers(ctx context.Context) (offers []Offer, err error) {
	defer mon.Task()(&ctx)(&err)

	offers, err = s.db.Offers().ListAll(ctx)
	if err != nil {
		return offers, Error.Wrap(err)
	}

	return offers, nil
}

// GetCurrentOffer returns current active offer
func (s *Service) GetCurrentOffer(ctx context.Context, offerStatus OfferStatus) (offer *Offer, err error) {
	defer mon.Task()(&ctx)(&err)

	offer, err = s.db.Offers().GetCurrent(ctx, offer.Status)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return offer, nil
}

// InsertNewOffer inserts a new offer into the db
func (s *Service) InsertNewOffer(ctx context.Context, offer *NewOffer) (o *Offer, err error) {
	defer mon.Task()(&ctx)(&err)

	if offer.Status == Default {
		offer.ExpiresAt = time.Now().UTC().AddDate(100, 0, 0)
		offer.RedeemableCap = 1
	}

	o, err = s.db.Offers().Create(ctx, offer)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return o, nil
}

// UpdateOffer modifies an existing offer in the db when the offer status is set to NoStatus
func (s *Service) UpdateOffer(ctx context.Context, o *UpdateOffer) (err error) {
	defer mon.Task()(&ctx)(&err)

	if o.Status == Default {
		o.NumRedeemed = 0
	}
	err = s.db.Offers().Update(ctx, o)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}
