package offers

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// DB holds information about offer
type DB interface {
	GetAllOffers(ctx context.Context) ([]Offer, error)
	Create(ctx context.Context, offer *Offer) error
	Update(ctx context.Context, offer *Offer) error
}

// Status indicates the status of an offer
type Status int

const (
	// NoStatus is a default offer status when no status is assigned during creation
	NoStatus Status = 0
	// OnGoing is a offer status when an offer is currently in use
	OnGoing Status = 1
	// Expired is a offer status when an offer passes it's duration setting
	Expired Status = 2
)

// OfferType indicates the type of an offer
type OfferType int

const (
	// FreeTier is a type of offers that's used for free credit program
	FreeTier OfferType = 0
	// Referral is a type of offers that's used for referral program
	Referral OfferType = 1
)

// Offer contains info needed for giving users free credits through different offer programs
type Offer struct {
	ID uuid.UUID `schema:"id"`

	Name        string `schema:"name, required"`
	Description string

	Credit int

	RedeemableCap int
	NumRedeemed   int

	OfferDuration         int
	AwardCreditDuration   int
	InviteeCreditDuration int
	CreatedAt             time.Time
	Status                Status
	Type                  OfferType
}
