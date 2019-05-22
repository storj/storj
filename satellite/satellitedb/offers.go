// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"
	"storj.io/storj/satellite/marketing"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type offers struct {
	db *dbx.DB
}

// ListAllOffers returns all offers from the db
func (offers *offers) ListAllOffers(ctx context.Context) ([]marketing.Offer, error) {
	offersDbx, err := offers.db.All_Offer(ctx)
	if err != nil {
		return nil, marketing.OffersErr.Wrap(err)
	}

	return offersFromDbx(offersDbx)
}

func (offers *offers) GetNoExpiredOffer(ctx context.Context, offerStatus marketing.OfferStatus, offerType marketing.OfferType) (*marketing.Offer, error) {
	if offerStatus == 0 || offerType == 0 {
		return nil, errs.New("offer status or type can't be nil")
	}

	offer, err := offers.db.Get_Offer_By_Status_And_Type_And_ExpiresAt_GreaterOrEqual(ctx, dbx.Offer_Status(int(offerStatus)), dbx.Offer_Type(int(offerType)), dbx.Offer_ExpiresAt(time.Now()))
	if err == sql.ErrNoRows {
		return nil, marketing.OffersErr.New("offer not found %i", offerStatus)
	}
	if err != nil {
		return nil, marketing.OffersErr.Wrap(err)
	}

	return convertDBOffer(offer)
}

// Create insert a new offer into the db
func (offers *offers) Create(ctx context.Context, offer *marketing.NewOffer) (*marketing.Offer, error) {
	createdOffer, err := offers.db.Create_Offer(ctx,
		dbx.Offer_Name(offer.Name),
		dbx.Offer_Description(offer.Description),
		dbx.Offer_Type(int(offer.Type)),
		dbx.Offer_CreditInCents(offer.CreditInCents),
		dbx.Offer_AwardCreditDurationDays(offer.AwardCreditDurationDays),
		dbx.Offer_InviteeCreditDurationDays(offer.InviteeCreditDurationDays),
		dbx.Offer_RedeemableCap(offer.RedeemableCap),
	)

	if err != nil {
		return nil, marketing.OffersErr.Wrap(err)
	}

	return convertDBOffer(createdOffer)
}

// Update modifies an existing offer
func (offers *offers) Update(ctx context.Context, offer *marketing.Offer) error {
	updateFields := dbx.Offer_Update_Fields{
		Name:                      dbx.Offer_Name(offer.Name),
		Description:               dbx.Offer_Description(offer.Description),
		Type:                      dbx.Offer_Type(int(offer.Type)),
		CreditInCents:             dbx.Offer_CreditInCents(offer.CreditInCents),
		AwardCreditDurationDays:   dbx.Offer_AwardCreditDurationDays(offer.AwardCreditDurationDays),
		InviteeCreditDurationDays: dbx.Offer_InviteeCreditDurationDays(offer.InviteeCreditDurationDays),
		RedeemableCap:             dbx.Offer_RedeemableCap(offer.RedeemableCap),
	}

	offerId := dbx.Offer_Id(offer.ID)

	tx, err := offers.db.Open(ctx)
	if err != nil {
		return marketing.OffersErr.Wrap(err)
	}

	currentOffer, err := tx.Get_Offer_By_Status_And_Type_And_ExpiresAt_GreaterOrEqual(ctx, updateFields.Status, updateFields.Type, dbx.Offer_ExpiresAt(time.Now()))
	if err == nil {
		statement := offers.db.Rebind(
			`UPDATE offers SET expires_at = NOW() WHERE offers.id=?`,
		)
		_, err = tx.Tx.ExecContext(ctx, statement, currentOffer.Id)
		if err != nil {
			return marketing.OffersErr.Wrap(errs.Combine(err, tx.Rollback()))
		}
	}

	if err != nil && err != sql.ErrNoRows {
		return marketing.OffersErr.Wrap(errs.Combine(err, tx.Rollback()))
	}

	_, err = tx.Update_Offer_By_Id_And_Status_Equal_Number_And_ExpiresAt_GreaterOrEqual_CreatedAt(ctx, offerId, updateFields)
	if err != nil {
		return marketing.OffersErr.Wrap(errs.Combine(err, tx.Rollback()))
	}

	return marketing.OffersErr.Wrap(tx.Commit())
}

// Delete is a method for deleting offer by Id from the database.
func (offers *offers) Delete(ctx context.Context, id int) error {
	_, err := offers.db.Delete_Offer_By_Id(ctx, dbx.Offer_Id(id))

	return marketing.OffersErr.Wrap(err)
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
		return nil, marketing.OffersErr.New("offerDbx parameter is nil")
	}

	o := marketing.Offer{
		ID:                        offerDbx.Id,
		Name:                      offerDbx.Name,
		Description:               offerDbx.Description,
		CreditInCents:             offerDbx.CreditInCents,
		RedeemableCap:             offerDbx.RedeemableCap,
		NumRedeemed:               offerDbx.NumRedeemed,
		ExpiresAt:                 *offerDbx.ExpiresAt,
		AwardCreditDurationDays:   offerDbx.AwardCreditDurationDays,
		InviteeCreditDurationDays: offerDbx.InviteeCreditDurationDays,
		CreatedAt:                 offerDbx.CreatedAt,
		Status:                    marketing.OfferStatus(offerDbx.Status),
		Type:                      marketing.OfferType(offerDbx.Type),
	}

	return &o, nil
}
