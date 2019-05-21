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
	ListAllOffers(ctx context.Context) ([]Offer, error)
	GetOfferByStatusAndType(ctx context.Context, offerStatus OfferStatus, offerType OfferType) (*Offer, error)
	Create(ctx context.Context, offer *Offer) (*Offer, error)
	Update(ctx context.Context, offer *Offer) error
	Delete(ctx context.Context, id int) error
}

// NewOffer holds information that's needed for creating a new offer
type NewOffer struct {
	Name        string
	Description string

	Credit int

	RedeemableCap int

	OfferDurationDays         int
	AwardCreditDurationDays   int
	InviteeCreditDurationDays int
	Type                      OfferType
}

// OfferStatus indicates the status of an offer
type OfferStatus int

const (
	// NoStatus is a default offer status when no status is assigned during creation
	NoStatus OfferStatus = 0
	// OnGoing is a offer status when an offer is currently in use
	OnGoing OfferStatus = 1
	// Expired is a offer status when an offer passes it's duration setting
	Expired OfferStatus = 2
)

// OfferType indicates the type of an offer
type OfferType int

const (
	// FreeTier is a type of offers that's used for free credit program
	FreeTier OfferType = 1
	// Referral is a type of offers that's used for referral program
	Referral OfferType = 2
)

// Offer contains info needed for giving users free credits through different offer programs
type Offer struct {
	ID int

	Name        string
	Description string

	Credit int

	RedeemableCap int
	NumRedeemed   int

	OfferDurationDays         int
	AwardCreditDurationDays   int
	InviteeCreditDurationDays int
	CreatedAt                 time.Time
	Status                    OfferStatus
	Type                      OfferType
}
