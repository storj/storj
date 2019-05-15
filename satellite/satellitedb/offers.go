package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"storj.io/storj/satellite/offers"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type offersDB struct {
	db dbx.Methods
}

// GetAllOffers returns all offers from the db
func (o *offersDB) GetAllOffers(ctx context.Context) ([]offers.Offer, error) {
	offersDbx, err := o.db.All_Offer(ctx)
	if err != nil {
		return nil, Errors.Wrap(err)
	}

	return offersFromDbx(offersDbx)
}

// Create insert a new offer into the db
func (o *offersDB) Create(ctx context.Context, offer *offers.Offer) (*offers.Offer, error) {
	offerID, err := uuid.New()
	if err != nil {
		return nil, Errors.Wrap(err)
	}

	createdOffer, err := o.db.Create_Offer(ctx,
		dbx.Offer_Id(offerID[:]),
		dbx.Offer_Name(offer.Name),
		dbx.Offer_Description(offer.Description),
		dbx.Offer_Type(int(offer.Type)),
		dbx.Offer_Credit(offer.Credit),
		dbx.Offer_AwardCreditDuration(offer.AwardCreditDuration),
		dbx.Offer_InviteeCreditDuration(offer.InviteeCreditDuration),
		dbx.Offer_RedeemableCap(offer.RedeemableCap),
		dbx.Offer_NumRedeemed(offer.NumRedeemed),
		dbx.Offer_OfferDuration(offer.OfferDuration),
		dbx.Offer_Status(int(offer.Status)),
	)

	if err != nil {
		return nil, Errors.Wrap(err)
	}

	return convertDBOffer(createdOffer)
}

// Update modifies an existing offer
func (o *offersDB) Update(ctx context.Context, offer *offers.Offer) error {
	updateFields := dbx.Offer_Update_Fields{
		Name:                  dbx.Offer_Name(offer.Name),
		Description:           dbx.Offer_Description(offer.Description),
		Type:                  dbx.Offer_Type(int(offer.Type)),
		Credit:                dbx.Offer_Credit(offer.Credit),
		AwardCreditDuration:   dbx.Offer_AwardCreditDuration(offer.AwardCreditDuration),
		InviteeCreditDuration: dbx.Offer_InviteeCreditDuration(offer.InviteeCreditDuration),
		RedeemableCap:         dbx.Offer_RedeemableCap(offer.RedeemableCap),
		NumRedeemed:           dbx.Offer_NumRedeemed(offer.NumRedeemed),
		OfferDuration:         dbx.Offer_OfferDuration(offer.OfferDuration),
		Status:                dbx.Offer_Status(int(offer.Status)),
	}

	offerId := dbx.Offer_Id(offer.ID[:])

	_, err := o.db.Update_Offer_By_Id(ctx, offerId, updateFields)

	return err

}

func offersFromDbx(offersDbx []*dbx.Offer) ([]offers.Offer, error) {
	var offers []offers.Offer
	var errors []error

	for _, offerDbx := range offersDbx {

		offer, err := convertDBOffer(offerDbx)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		offers = append(offers, *offer)
	}

	return offers, errs.Combine(errors...)
}

func convertDBOffer(offerDbx *dbx.Offer) (*offers.Offer, error) {
	if offerDbx == nil {
		return nil, errs.New("offerDbx parameter is nil")
	}

	id, err := bytesToUUID(offerDbx.Id)
	if err != nil {
		return nil, err
	}

	o := offers.Offer{
		ID:                    id,
		Name:                  offerDbx.Name,
		Description:           offerDbx.Description,
		Credit:                offerDbx.Credit,
		RedeemableCap:         offerDbx.RedeemableCap,
		NumRedeemed:           offerDbx.NumRedeemed,
		OfferDuration:         offerDbx.OfferDuration,
		AwardCreditDuration:   offerDbx.AwardCreditDuration,
		InviteeCreditDuration: offerDbx.InviteeCreditDuration,
		CreatedAt:             offerDbx.CreatedAt,
		Status:                offers.Status(offerDbx.Status),
		Type:                  offers.OfferType(offerDbx.Type),
	}

	return &o, nil
}
