// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package rewards

import (
	"context"
	"time"

	"storj.io/storj/internal/currency"
)

// DB holds information about offer
type DB interface {
	ListAll(ctx context.Context) ([]Offer, error)
	GetCurrentByType(ctx context.Context, offerType OfferType) (*Offer, error)
	Create(ctx context.Context, offer *NewOffer) (*Offer, error)
	Redeem(ctx context.Context, offerID int, isDefault bool) error
	Finish(ctx context.Context, offerID int) error
}

// NewOffer holds information that's needed for creating a new offer
type NewOffer struct {
	Name        string
	Description string

	AwardCredit   currency.USD
	InviteeCredit currency.USD

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

	AwardCredit   currency.USD
	InviteeCredit currency.USD

	AwardCreditDurationDays   int
	InviteeCreditDurationDays int

	RedeemableCap int
	NumRedeemed   int

	ExpiresAt time.Time
	CreatedAt time.Time

	Status OfferStatus
	Type   OfferType
}

// IsDefault evaluates the default status of offers for templates.
func (o Offer) IsDefault() bool {
	if o.Status == Default {
		return true
	}
	return false
}

// IsCurrent evaluates the current status of offers for templates.
func (o Offer) IsCurrent() bool {
	if o.Status == Active {
		return true
	}
	return false
}

// IsDone evaluates the done status of offers for templates.
func (o Offer) IsDone() bool {
	if o.Status == Done {
		return true
	}
	return false
}

// Offers holds a set of organized offers.
type Offers struct {
	Set []Offer
}

// GetCurrentFromSet returns the current offer from an organized set.
func (offers Offers) GetCurrentFromSet() Offer {
	var o Offer
	for _, offer := range offers.Set {
		if offer.IsCurrent() {
			o = offer
		}
	}
	return o
}

// GetDefaultFromSet returns the current offer from an organized set.
func (offers Offers) GetDefaultFromSet() Offer {
	var o Offer
	for _, offer := range offers.Set {
		if offer.IsDefault() {
			o = offer
		}
	}
	return o
}
