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
