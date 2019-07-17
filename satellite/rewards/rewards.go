// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package rewards

import (
	"context"
	"time"

	"storj.io/storj/internal/currency"
	"storj.io/storj/satellite/partners"
)

// MaxRedemptionErr is the error message used when an offer has reached its redemption capacity
var MaxRedemptionErr = "This offer redemption has reached its capacity"

// DB holds information about offer
type DB interface {
	ListAll(ctx context.Context) (Offers, error)
	GetCurrentByType(ctx context.Context, offerType OfferType) (*Offer, error)
	Create(ctx context.Context, offer *NewOffer) (*Offer, error)
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
	// Invalid is a default value for offers that don't have correct type associated with it
	Invalid = OfferType(0)
	// FreeCredit is a type of offers used for Free Credit Program
	FreeCredit = OfferType(1)
	// Referral is a type of offers used for Referral Program
	Referral = OfferType(2)
	// Partner is an OfferType used be the Open Source Partner Program
	Partner = OfferType(3)
)

// OfferStatus represents the different stage an offer can have in its life-cycle.
type OfferStatus int

const (

	// Done is the status of an offer that is no longer in use.
	Done = OfferStatus(iota)

	// Active is the status of an offer that is currently in use.
	Active

	// Default is the status of an offer when there is no active offer.
	Default
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

	ExpiresAt time.Time
	CreatedAt time.Time

	Status OfferStatus
	Type   OfferType
}

// IsEmpty evaluates whether or not an on offer is empty
func (o Offer) IsEmpty() bool {
	return o.Name == ""
}

// Offers contains a slice of offers.
type Offers []Offer

// OrganizedOffers contains a list of offers organized by status.
type OrganizedOffers struct {
	Active  Offer
	Default Offer
	Done    Offers
}

// OpenSourcePartner contains all data for an Open Source Partner.
type OpenSourcePartner struct {
	Name          string
	ID            string
	Offers        Offers
	PartnerOffers OrganizedOffers
}

// PartnerSet contains a list of Open Source Partners.
type PartnerSet []OpenSourcePartner

// OfferSet provides a separation of marketing offers by type.
type OfferSet struct {
	ReferralOffers OrganizedOffers
	FreeCredits    OrganizedOffers
	PartnerTables  PartnerSet
}

// OrganizeOffersByStatus organizes offers by OfferStatus.
func (offers Offers) OrganizeOffersByStatus() OrganizedOffers {
	var oo OrganizedOffers

	for _, offer := range offers {
		switch offer.Status {
		case Active:
			oo.Active = offer
		case Default:
			oo.Default = offer
		case Done:
			oo.Done = append(oo.Done, offer)
		}
	}
	return oo
}

// OrganizeOffersByType organizes offers by OfferType.
func (offers Offers) OrganizeOffersByType() OfferSet {
	var (
		fc, ro, p Offers
		offerSet  OfferSet
	)

	for _, offer := range offers {
		switch offer.Type {
		case FreeCredit:
			fc = append(fc, offer)
		case Referral:
			ro = append(ro, offer)
		case Partner:
			p = append(p, offer)
		default:
			continue
		}
	}

	offerSet.FreeCredits = fc.OrganizeOffersByStatus()
	offerSet.ReferralOffers = ro.OrganizeOffersByStatus()
	offerSet.PartnerTables = OrganizePartnerData(p)
	return offerSet
}

// CreatePartnerSet generates a PartnerSet from the config file.
func CreatePartnerSet() PartnerSet {
	partners := partners.LoadPartners()
	var ps PartnerSet
	for _, partner := range partners {
		ps = append(ps, OpenSourcePartner{
			Name:   partner.Name,
			ID:     partner.ID,
			Offers: Offers{},
		})
	}
	return ps
}

// MatchOffersToPartnerSet assigns offers to the partner they belong to.
func MatchOffersToPartnerSet(offers Offers, partnerSet PartnerSet) PartnerSet {
	for _, o := range offers {
		for index, p := range partnerSet {
			if o.Name == p.ID+"-"+p.Name {
				p.Offers = append(p.Offers, o)
				partnerSet[index].Offers = append(partnerSet[index].Offers, o)
			}
		}
	}

	for index, partner := range partnerSet {
		partnerSet[index].PartnerOffers = partner.Offers.OrganizeOffersByStatus()
	}
	return partnerSet
}

// OrganizePartnerData returns a list of Open Source Partners
// whos offers have been organized by status, type, and
// assigned to the correct partner.
func OrganizePartnerData(offers Offers) PartnerSet {
	partnerData := MatchOffersToPartnerSet(offers, CreatePartnerSet())
	return partnerData
}
