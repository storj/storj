package marketing

import (
	"context"
	"database/sql"

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

	offers, err = s.db.Offers().ListAllOffers(ctx)
	if err != nil {
		return offers, Error.Wrap(err)
	}

	return
}

// GetCurrentOfferByType returns currently on-going offer based on offer type
func (s *Service) GetCurrentOfferByType(ctx context.Context, offerType OfferType) (offer *Offer, err error) {
	defer mon.Task()(&ctx)(&err)

	offer, err = s.db.Offers().GetNoExpiredOffer(ctx, OnGoing, offerType)
	if err == sql.ErrNoRows && offerType == Referral {
		offer, err = s.db.Offers().GetNoExpiredOffer(ctx, OnGoing, offerType)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return offer, nil
}

// InsertNewOffer inserts a new offer into the db
func (s *Service) InsertNewOffer(ctx context.Context, offer *NewOffer) (o *Offer, err error) {
	defer mon.Task()(&ctx)(&err)

	o, err = s.db.Offers().Create(ctx, offer)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return o, nil
}

// UpdateOffer modifies an existing offer in the db when the offer status is set to NoStatus
func (s *Service) UpdateOffer(ctx context.Context, offer *Offer) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.db.Offers().Update(ctx, offer)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}
