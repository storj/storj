// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package satellitedb

import (
	"context"
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/storj/satellite/marketing"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type offers struct {
	db *dbx.DB
}

// ListAll returns all offers from the db
func (offers *offers) ListAll(ctx context.Context) ([]marketing.Offer, error) {
	offersDbx, err := offers.db.All_Offer(ctx)
	if err != nil {
		return nil, marketing.OffersErr.Wrap(err)
	}

	return offersFromDbx(offersDbx)
}

// GetCurrent returns an offer that has not expired based on offer status
func (offers *offers) GetCurrent(ctx context.Context) (*marketing.Offer, error) {
	statement := offers.db.Rebind(
		`WITH o AS (
			SELECT * FROM offers WHERE offers.status=? AND offers.expires_at>NOW()
		  )
		  SELECT * FROM o
		  UNION ALL
		  SELECT * FROM offers
		  WHERE offers.status=?
		  AND NOT EXISTS (
			SELECT * FROM o
		  ) order by offers.created_at desc;`)

	rows, err := offers.db.DB.QueryContext(ctx, statement, marketing.Active, marketing.Default)
	if err == sql.ErrNoRows {
		return nil, marketing.OffersErr.New("no current offer")
	}
	if err != nil {
		return nil, marketing.OffersErr.Wrap(err)
	}

	o := marketing.Offer{}
	err = rows.Scan(&o)
	if err != nil {
		return nil, errs.Combine(marketing.OffersErr.Wrap(err), rows.Close())
	}
	return &o, nil
}

// Create inserts a new offer into the db
func (offers *offers) Create(ctx context.Context, o *marketing.NewOffer) (*marketing.Offer, error) {
	tx, err := offers.db.Open(ctx)
	if err != nil {
		return nil, marketing.OffersErr.Wrap(err)
	}

	statement := offers.db.Rebind(`
		UPDATE offers SET offers.status=?, offers.expires_at=NOW()
		WHERE offers.status=? AND expires_at>NOW();
	`)
	_, err = tx.Tx.ExecContext(ctx, statement, marketing.Done, o.Status)
	if err != nil {
		return nil, marketing.OffersErr.Wrap(errs.Combine(err, tx.Rollback()))
	}

	offerDbx, err := tx.Create_Offer(ctx,
		dbx.Offer_Name(o.Name),
		dbx.Offer_Description(o.Description),
		dbx.Offer_AwardCreditInCents(o.AwardCreditInCents),
		dbx.Offer_InviteeCreditInCents(o.InviteeCreditInCents),
		dbx.Offer_AwardCreditDurationDays(o.AwardCreditDurationDays),
		dbx.Offer_InviteeCreditDurationDays(o.InviteeCreditDurationDays),
		dbx.Offer_RedeemableCap(o.RedeemableCap),
		dbx.Offer_ExpiresAt(o.ExpiresAt),
		dbx.Offer_Status(int(o.Status)),
	)
	if err != nil {
		return nil, marketing.OffersErr.Wrap(errs.Combine(err, tx.Rollback()))
	}

	newOffer, err := convertDBOffer(offerDbx)
	if err != nil {
		return nil, marketing.OffersErr.Wrap(errs.Combine(err, tx.Rollback()))
	}

	return newOffer, marketing.OffersErr.Wrap(tx.Commit())
}

// Update modifies an offer entry's status and amount of offers redeemed based on offer id
func (offers *offers) Update(ctx context.Context, id int, o *marketing.UpdateOffer) error {
	updateFields := dbx.Offer_Update_Fields{
		Status:      dbx.Offer_Status(int(o.Status)),
		NumRedeemed: dbx.Offer_NumRedeemed(o.NumRedeemed),
		ExpiresAt:   dbx.Offer_ExpiresAt(o.ExpiresAt),
	}

	offerId := dbx.Offer_Id(id)

	_, err := offers.db.Update_Offer_By_Id(ctx, offerId, updateFields)
	if err != nil {
		return marketing.OffersErr.Wrap(err)
	}

	return nil
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
		AwardCreditInCents:        offerDbx.AwardCreditInCents,
		InviteeCreditInCents:      offerDbx.InviteeCreditInCents,
		RedeemableCap:             offerDbx.RedeemableCap,
		NumRedeemed:               offerDbx.NumRedeemed,
		ExpiresAt:                 offerDbx.ExpiresAt,
		AwardCreditDurationDays:   offerDbx.AwardCreditDurationDays,
		InviteeCreditDurationDays: offerDbx.InviteeCreditDurationDays,
		CreatedAt:                 offerDbx.CreatedAt,
		Status:                    marketing.OfferStatus(offerDbx.Status),
	}

	return &o, nil
}
