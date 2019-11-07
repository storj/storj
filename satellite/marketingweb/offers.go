// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package marketingweb

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/satellite/rewards"
)

// OrganizedOffers contains a list of offers organized by status.
type OrganizedOffers struct {
	Active  rewards.Offer
	Default rewards.Offer
	Done    rewards.Offers
}

// OpenSourcePartner contains all data for an Open Source Partner.
type OpenSourcePartner struct {
	rewards.PartnerInfo
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
func (server *Server) OrganizeOffersByStatus(offers rewards.Offers) OrganizedOffers {
	var oo OrganizedOffers

	for _, offer := range offers {
		switch offer.Status {
		case rewards.Active:
			if !oo.Active.IsZero() {
				server.log.Error("duplicate active")
			}
			oo.Active = offer
		case rewards.Default:
			if !oo.Active.IsZero() {
				server.log.Error("duplicate default")
			}
			oo.Default = offer
		case rewards.Done:
			oo.Done = append(oo.Done, offer)
		}
	}
	return oo
}

// OrganizeOffersByType organizes offers by OfferType.
func (server *Server) OrganizeOffersByType(offers rewards.Offers) OfferSet {
	var (
		fc, ro, p rewards.Offers
		offerSet  OfferSet
	)

	for _, offer := range offers {
		switch offer.Type {
		case rewards.FreeCredit:
			fc = append(fc, offer)
		case rewards.Referral:
			ro = append(ro, offer)
		case rewards.Partner:
			p = append(p, offer)
		default:
			continue
		}
	}

	offerSet.FreeCredits = server.OrganizeOffersByStatus(fc)
	offerSet.ReferralOffers = server.OrganizeOffersByStatus(ro)
	offerSet.PartnerTables = server.organizePartnerData(p)
	return offerSet
}

// createPartnerSet generates a PartnerSet from the config file.
func (server *Server) createPartnerSet() PartnerSet {
	all, err := server.partners.All(context.TODO()) // TODO: don't ignore error
	if err != nil {
		server.log.Error("failed to load all partners", zap.Error(err))
		return nil
	}

	var ps PartnerSet
	for _, partner := range all {
		ps = append(ps, OpenSourcePartner{
			PartnerInfo: partner,
		})
	}
	return ps
}

// matchOffersToPartnerSet assigns offers to the partner they belong to.
func (server *Server) matchOffersToPartnerSet(offers rewards.Offers, partnerSet PartnerSet) PartnerSet {
	for i := range partnerSet {
		var partnerOffersByName rewards.Offers

		for _, o := range offers {
			if o.Name == partnerSet[i].PartnerInfo.Name {
				partnerOffersByName = append(partnerOffersByName, o)
			}
		}

		partnerSet[i].PartnerOffers = server.OrganizeOffersByStatus(partnerOffersByName)
	}

	return partnerSet
}

// organizePartnerData returns a list of Open Source Partners
// whose offers have been organized by status, type, and
// assigned to the correct partner.
func (server *Server) organizePartnerData(offers rewards.Offers) PartnerSet {
	partnerData := server.matchOffersToPartnerSet(offers, server.createPartnerSet())
	return partnerData
}
