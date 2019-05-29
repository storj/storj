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

// ListAll returns all offers from the db
func (offers *offers) ListAll(ctx context.Context) ([]marketing.Offer, error) {
	offersDbx, err := offers.db.All_Offer(ctx)
	if err != nil {
		return nil, marketing.OffersErr.Wrap(err)
	}

	return offersFromDbx(offersDbx)
}

// GetCurrent returns an offer that has not expired based on offer status
func (offers *offers) GetCurrent(ctx context.Context, isDefault bool) (*marketing.Offer, error) {
	var statement string
	const columns = "id, name, description, award_credit_in_cents, invitee_credit_in_cents, award_credit_duration_days, invitee_credit_duration_days, redeemable_cap, num_redeemed, expires_at, created_at, status"
	if isDefault {
		statement = `SELECT ` + columns + ` FROM offers
			WHERE status=? AND expires_at>?
			order by created_at desc;`
	} else {
		statement = `
			WITH o AS (
				SELECT ` + columns + ` FROM offers WHERE status=? AND expires_at>?
			  )
			  SELECT ` + columns + ` FROM o
			  UNION ALL
			  SELECT ` + columns + ` FROM offers
			  WHERE status=?
			  AND NOT EXISTS (
				SELECT id FROM o
			  ) order by created_at desc;`
	}

	o := marketing.Offer{}
	rows := offers.db.DB.QueryRowContext(ctx, offers.db.Rebind(statement), marketing.Active, time.Now(), marketing.Default)
	err := rows.Scan(&o.ID, &o.Name, &o.Description, &o.AwardCreditInCents, &o.InviteeCreditInCents, &o.AwardCreditDurationDays, &o.InviteeCreditDurationDays, &o.RedeemableCap, &o.NumRedeemed, &o.ExpiresAt, &o.CreatedAt, &o.Status)
	if err == sql.ErrNoRows {
		return nil, marketing.OffersErr.New("no current offer")
	}
	if err != nil {
		return nil, marketing.OffersErr.Wrap(err)
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
		UPDATE offers SET status=?, expires_at=?
		WHERE status=? AND expires_at>?;
	`)
	currentTime := time.Now()
	_, err = tx.Tx.ExecContext(ctx, statement, marketing.Done, currentTime, o.Status, currentTime)
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
func (offers *offers) Update(ctx context.Context, o *marketing.UpdateOffer) error {
	updateFields := dbx.Offer_Update_Fields{
		Status:      dbx.Offer_Status(int(o.Status)),
		NumRedeemed: dbx.Offer_NumRedeemed(o.NumRedeemed),
		ExpiresAt:   dbx.Offer_ExpiresAt(o.ExpiresAt),
	}

	offerID := dbx.Offer_Id(o.ID)

	_, err := offers.db.Update_Offer_By_Id(ctx, offerID, updateFields)
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
