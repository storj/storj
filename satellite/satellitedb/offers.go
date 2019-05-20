package satellitedb

import (
	"context"
	"database/sql"

	"github.com/zeebo/errs"
	"storj.io/storj/satellite/marketing"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type offersDB struct {
	db dbx.Methods
}

// GetAllOffers returns all offers from the db
func (o *offersDB) GetAllOffers(ctx context.Context) ([]marketing.Offer, error) {
	offersDbx, err := o.db.All_Offer(ctx)
	if err != nil {
		return nil, marketing.ErrOffers.Wrap(err)
	}

	return offersFromDbx(offersDbx)
}

func (o *offersDB) GetOfferByStatusAndType(ctx context.Context, offerStatus marketing.OfferStatus, offerType marketing.OfferType) (*marketing.Offer, error) {
	if offerStatus == 0 || offerType == 0 {
		return nil, errs.New("offer status or type can't be nil")
	}

	offer, err := o.db.Get_Offer_By_Status_And_Type(ctx, dbx.Offer_Status(int(offerStatus)), dbx.Offer_Type(int(offerType)))
	if err == sql.ErrNoRows {
		return nil, marketing.ErrOffers.New("not found %v", offerStatus)
	}
	if err != nil {
		return nil, marketing.ErrOffers.Wrap(err)
	}

	return convertDBOffer(offer)
}

// Create insert a new offer into the db
func (o *offersDB) Create(ctx context.Context, offer *marketing.Offer) (*marketing.Offer, error) {
	createdOffer, err := o.db.Create_Offer(ctx,
		dbx.Offer_Name(offer.Name),
		dbx.Offer_Description(offer.Description),
		dbx.Offer_Type(int(offer.Type)),
		dbx.Offer_Credit(offer.Credit),
		dbx.Offer_AwardCreditDurationDays(offer.AwardCreditDurationDays),
		dbx.Offer_InviteeCreditDurationDays(offer.InviteeCreditDurationDays),
		dbx.Offer_RedeemableCap(offer.RedeemableCap),
		dbx.Offer_NumRedeemed(offer.NumRedeemed),
		dbx.Offer_OfferDurationDays(offer.OfferDurationDays),
		dbx.Offer_Status(int(offer.Status)),
	)

	if err != nil {
		return nil, marketing.ErrOffers.Wrap(err)
	}

	return convertDBOffer(createdOffer)
}

// Update modifies an existing offer
func (o *offersDB) Update(ctx context.Context, offer *marketing.Offer) error {
	updateFields := dbx.Offer_Update_Fields{
		Name:                      dbx.Offer_Name(offer.Name),
		Description:               dbx.Offer_Description(offer.Description),
		Type:                      dbx.Offer_Type(int(offer.Type)),
		Credit:                    dbx.Offer_Credit(offer.Credit),
		AwardCreditDurationDays:   dbx.Offer_AwardCreditDurationDays(offer.AwardCreditDurationDays),
		InviteeCreditDurationDays: dbx.Offer_InviteeCreditDurationDays(offer.InviteeCreditDurationDays),
		RedeemableCap:             dbx.Offer_RedeemableCap(offer.RedeemableCap),
		NumRedeemed:               dbx.Offer_NumRedeemed(offer.NumRedeemed),
		OfferDurationDays:         dbx.Offer_OfferDurationDays(offer.OfferDurationDays),
		Status:                    dbx.Offer_Status(int(offer.Status)),
	}

	offerId := dbx.Offer_Id(offer.ID)

	_, err := o.db.Update_Offer_By_Id(ctx, offerId, updateFields)

	return marketing.ErrOffers.Wrap(err)

}

// Delete is a method for deleting offer by Id from the database.
func (o *offersDB) Delete(ctx context.Context, id int) error {
	_, err := o.db.Delete_Offer_By_Id(ctx, dbx.Offer_Id(id))

	return marketing.ErrOffers.Wrap(err)
}

func offersFromDbx(offersDbx []*dbx.Offer) ([]marketing.Offer, error) {
	var offers []marketing.Offer
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

func convertDBOffer(offerDbx *dbx.Offer) (*marketing.Offer, error) {
	if offerDbx == nil {
		return nil, marketing.ErrOffers.New("offerDbx parameter is nil")
	}

	o := marketing.Offer{
		ID:                        offerDbx.Id,
		Name:                      offerDbx.Name,
		Description:               offerDbx.Description,
		Credit:                    offerDbx.Credit,
		RedeemableCap:             offerDbx.RedeemableCap,
		NumRedeemed:               offerDbx.NumRedeemed,
		OfferDurationDays:         offerDbx.OfferDurationDays,
		AwardCreditDurationDays:   offerDbx.AwardCreditDurationDays,
		InviteeCreditDurationDays: offerDbx.InviteeCreditDurationDays,
		CreatedAt:                 offerDbx.CreatedAt,
		Status:                    marketing.OfferStatus(offerDbx.Status),
		Type:                      marketing.OfferType(offerDbx.Type),
	}

	return &o, nil
}
