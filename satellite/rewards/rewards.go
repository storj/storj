// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package rewards

import (
	"context"
	"time"

	"storj.io/storj/internal/currency"
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

// FormatPartnerName formats partner's name into combination of its partnerID and name
func (o NewOffer) FormatPartnerName() string {
	if o.Type != Partner {
		return o.Name
	}

	partnerInfo := PartnerInfo{
		ID:   LoadPartnerInfos()[o.Name].ID,
		Name: o.Name,
	}
	return partnerInfo.FormattedName()
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
	// Partner is a type of offers used for Open Source Partner Program
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
	PartnerInfo
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
	offerSet.PartnerTables = organizePartnerData(p)
	return offerSet
}

// createPartnerSet generates a PartnerSet from the config file.
func createPartnerSet() PartnerSet {
	partners := LoadPartnerInfos()
	var ps PartnerSet
	for _, partner := range partners {
		ps = append(ps, OpenSourcePartner{
			PartnerInfo: PartnerInfo{
				Name: partner.Name,
				ID:   partner.ID,
			},
		})
	}
	return ps
}

// matchOffersToPartnerSet assigns offers to the partner they belong to.
func matchOffersToPartnerSet(offers Offers, partnerSet PartnerSet) PartnerSet {
	for i := range partnerSet {
		var partnerOffersByName Offers

		for _, o := range offers {
			if o.Name == partnerSet[i].PartnerInfo.FormattedName() {
				partnerOffersByName = append(partnerOffersByName, o)
			}
		}

		partnerSet[i].PartnerOffers = partnerOffersByName.OrganizeOffersByStatus()
	}

	return partnerSet
}

// organizePartnerData returns a list of Open Source Partners
// whose offers have been organized by status, type, and
// assigned to the correct partner.
func organizePartnerData(offers Offers) PartnerSet {
	partnerData := matchOffersToPartnerSet(offers, createPartnerSet())
	return partnerData
}
