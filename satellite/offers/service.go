package offers

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	// Error the default offers errs class
	Error = errs.Class("offers error")

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
		return nil, errs.New("log can't be nil")
	}

	return &Service{
		log: log,
		db:  db,
	}, nil
}

// ListAllOffers returns all available offers in the db
func (s *Service) ListAllOffers(ctx context.Context) (offers []Offer, err error) {
	defer mon.Task()(&ctx)(&err)

	offers, err = s.db.GetAllOffers(ctx)
	if err != nil {
		return offers, Error.Wrap(err)
	}

	return
}

// Create inserts a new offer into the db
func (s *Service) Create(ctx context.Context, offer *Offer) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.db.Create(ctx, offer)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// Update modifies an existing offer in the db
func (s *Service) Update(ctx context.Context, offer *Offer) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.db.Update(ctx, offer)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}
