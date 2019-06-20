// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package offers

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"
)

// Err creates offer error class
var Err = errs.Class("offers error")

// DB holds information about offer
type DB interface {
	ListAll(ctx context.Context) ([]Offer, error)
	GetCurrentByType(ctx context.Context, offerType OfferType) (*Offer, error)
	Create(ctx context.Context, offer *NewOffer) (*Offer, error)
	Redeem(ctx context.Context, offerID int) error
	Finish(ctx context.Context, offerID int) error
}

// NewOffer holds information that's needed for creating a new offer
type NewOffer struct {
	Name        string
	Description string

	AwardCreditInCents   int
	InviteeCreditInCents int

	RedeemableCap int

	AwardCreditDurationDays   int
	InviteeCreditDurationDays int

	ExpiresAt time.Time

	Status OfferStatus
	Type   OfferType
}

// UpdateOffer holds fields needed for update an offer
type UpdateOffer struct {
	ID        int
	Status    OfferStatus
	ExpiresAt time.Time
}

// OfferType indicates the type of an offer
type OfferType int

const (
	// FreeCredit is a type of offers used for Free Credit Program
	FreeCredit = OfferType(iota)
	// Referral is a type of offers used for Referral Program
	Referral
)

// OfferStatus indicates the status of an offer
type OfferStatus int

const (
	// Done is a default offer status when an offer is not being used currently
	Done = OfferStatus(iota)
	// Default is a offer status when an offer is used as a default offer
	Default
	// Active is a offer status when an offer is currently being used
	Active
)

// Offer contains info needed for giving users free credits through different offer programs
type Offer struct {
	ID          int
	Name        string
	Description string

	AwardCreditInCents   int
	InviteeCreditInCents int

	AwardCreditDurationDays   int
	InviteeCreditDurationDays int

	RedeemableCap int
	NumRedeemed   int

	ExpiresAt time.Time
	CreatedAt time.Time

	Status OfferStatus
	Type   OfferType
}

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

	offers, err = s.db.ListAll(ctx)
	if err != nil {
		return offers, Error.Wrap(err)
	}

	return offers, nil
}

// GetCurrentOfferByType returns current active offer
func (s *Service) GetCurrentOfferByType(ctx context.Context, offerType OfferType) (offer *Offer, err error) {
	defer mon.Task()(&ctx)(&err)

	offer, err = s.db.GetCurrentByType(ctx, offerType)
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

	o, err = s.db.Create(ctx, offer)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return o, nil
}

// RedeemOffer adds 1 to the number of redeemed for an offer
func (s *Service) RedeemOffer(ctx context.Context, uo *UpdateOffer) (err error) {
	defer mon.Task()(&ctx)(&err)

	if uo.Status == Default {
		return nil
	}

	err = s.db.Redeem(ctx, uo.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// FinishOffer updates an active offer's status to be Done and its expiration time to be now
func (s *Service) FinishOffer(ctx context.Context, oID int) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.db.Finish(ctx, oID)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}
