// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package marketing

import (
	"context"
	"time"

	"github.com/zeebo/errs"
)

// OffersErr creates offer error class
var OffersErr = errs.Class("offers error")

// Offers holds information about offer
type Offers interface {
	ListAll(ctx context.Context) ([]Offer, error)
	GetCurrent(ctx context.Context) (*Offer, error)
	Create(ctx context.Context, offer *NewOffer) (*Offer, error)
	Update(ctx context.Context, id int, offer *UpdateOffer) error
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
}

// UpdateOffer holds fields that can be updated
type UpdateOffer struct {
	Status      OfferStatus
	NumRedeemed int
	ExpiresAt   time.Time
}

// OfferStatus indicates the status of an offer
type OfferStatus int

const (
	// Done is a default offer status when an offer is not being used currently
	Done OfferStatus = 0
	// Default is a offer status when an offer is used as a default offer
	Default OfferStatus = 1
	// Active is a offer status when an offer is currently being used
	Active OfferStatus = 2
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
}
