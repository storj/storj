// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/currency"
	"storj.io/storj/satellite/rewards"
)

// NoCreditForUpdateErr is a error message used when no credits are found for update when new users sign up
var NoCreditForUpdateErr = errs.Class("no credit found to update")

// UserCredits holds information to interact with database
//
// architecture: Database
type UserCredits interface {
	GetCreditUsage(ctx context.Context, userID uuid.UUID, expirationEndDate time.Time) (*UserCreditUsage, error)
	Create(ctx context.Context, userCredit CreateCredit) error
	UpdateEarnedCredits(ctx context.Context, userID uuid.UUID) error
	UpdateAvailableCredits(ctx context.Context, creditsToCharge int, id uuid.UUID, billingStartDate time.Time) (remainingCharge int, err error)
}

// CreditType indicates a type of a credit
type CreditType string

const (
	// Invitee is a type of credits earned by invitee
	Invitee CreditType = "invitee"
	// Referrer is a type of credits earned by referrer
	Referrer CreditType = "referrer"
)

// UserCredit holds information about an user's credit
type UserCredit struct {
	ID            int
	UserID        uuid.UUID
	OfferID       int
	ReferredBy    *uuid.UUID
	Type          CreditType
	CreditsEarned currency.USD
	CreditsUsed   currency.USD
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

// UserCreditUsage holds information about credit usage information
type UserCreditUsage struct {
	Referred         int64
	AvailableCredits currency.USD
	UsedCredits      currency.USD
}

// CreateCredit holds information that's needed when create a new record of user credit
type CreateCredit struct {
	OfferInfo     rewards.RedeemOffer
	UserID        uuid.UUID
	OfferID       int
	Type          CreditType
	ReferredBy    *uuid.UUID
	CreditsEarned currency.USD
	ExpiresAt     time.Time
}

// NewCredit returns a new credit data
func NewCredit(currentReward *rewards.Offer, creditType CreditType, userID uuid.UUID, referrerID *uuid.UUID) (*CreateCredit, error) {
	var creditEarned currency.USD
	switch creditType {
	case Invitee:
		// Invitee will only earn their credit once they have activated their account. Therefore, we set it to 0 on creation
		creditEarned = currency.Cents(0)
	case Referrer:
		creditEarned = currentReward.AwardCredit
	default:
		return nil, errs.New("unsupported credit type")
	}
	return &CreateCredit{
		OfferInfo: rewards.RedeemOffer{
			RedeemableCap: currentReward.RedeemableCap,
			Status:        currentReward.Status,
			Type:          currentReward.Type,
		},
		UserID:        userID,
		OfferID:       currentReward.ID,
		ReferredBy:    referrerID,
		CreditsEarned: creditEarned,
		ExpiresAt:     time.Now().UTC().AddDate(0, 0, currentReward.InviteeCreditDurationDays),
	}, nil
}
