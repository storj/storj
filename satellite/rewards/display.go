// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package rewards

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

type referralInfo struct {
	UserID    string
	PartnerID string
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
			PartnerInfo: partner,
		})
	}
	return ps
}

// matchOffersToPartnerSet assigns offers to the partner they belong to.
func matchOffersToPartnerSet(offers Offers, partnerSet PartnerSet) PartnerSet {
	for i := range partnerSet {
		var partnerOffersByName Offers

		for _, o := range offers {
			if o.Name == partnerSet[i].PartnerInfo.Name {
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

// getTardigradeDomains returns domain names for tardigrade satellites
func getTardigradeDomains() []string {
	return []string{
		"https://us-central-1.tardigrade.io/",
		"https://asia-east-1.tardigrade.io/",
		"https://europe-west-1.tardigrade.io/",
	}
}
