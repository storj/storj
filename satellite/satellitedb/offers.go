// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	offers2 "storj.io/storj/satellite/offers"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type offersDB struct {
	db *dbx.DB
}

// ListAll returns all offersDB from the db
func (offers *offersDB) ListAll(ctx context.Context) ([]offers2.Offer, error) {
	offersDbx, err := offers.db.All_Offer(ctx)
	if err != nil {
		return nil, offers2.Err.Wrap(err)
	}

	return offersFromDBX(offersDbx)
}

// GetCurrent returns an offer that has not expired based on offer type
func (offers *offersDB) GetCurrentByType(ctx context.Context, offerType offers2.OfferType) (*offers2.Offer, error) {
	var statement string
	const columns = "id, name, description, award_credit_in_cents, invitee_credit_in_cents, award_credit_duration_days, invitee_credit_duration_days, redeemable_cap, num_redeemed, expires_at, created_at, status, type"
	statement = `
		WITH o AS (
			SELECT ` + columns + ` FROM offersDB WHERE status=? AND type=? AND expires_at>? AND num_redeemed < redeemable_cap
		)
		SELECT ` + columns + ` FROM o
		UNION ALL
		SELECT ` + columns + ` FROM offersDB
		WHERE type=? AND status=?
		AND NOT EXISTS (
			SELECT id FROM o
		) order by created_at desc;`

	rows := offers.db.DB.QueryRowContext(ctx, offers.db.Rebind(statement), offers2.Active, offerType, time.Now().UTC(), offerType, offers2.Default)

	o := offers2.Offer{}
	err := rows.Scan(&o.ID, &o.Name, &o.Description, &o.AwardCreditInCents, &o.InviteeCreditInCents, &o.AwardCreditDurationDays, &o.InviteeCreditDurationDays, &o.RedeemableCap, &o.NumRedeemed, &o.ExpiresAt, &o.CreatedAt, &o.Status, &o.Type)
	if err == sql.ErrNoRows {
		return nil, offers2.Err.New("no current offer")
	}
	if err != nil {
		return nil, offers2.Err.Wrap(err)
	}

	return &o, nil
}

// Create inserts a new offer into the db
func (offers *offersDB) Create(ctx context.Context, o *offers2.NewOffer) (*offers2.Offer, error) {
	currentTime := time.Now()
	if o.ExpiresAt.Before(currentTime) {
		return nil, offers2.Err.New("expiration time: %v can't be before: %v", o.ExpiresAt, currentTime)
	}

	tx, err := offers.db.Open(ctx)
	if err != nil {
		return nil, offers2.Err.Wrap(err)
	}

	// If there's an existing current offer, update its status to Done and set its expires_at to be NOW()
	statement := offers.db.Rebind(`
		UPDATE offersDB SET status=?, expires_at=?
		WHERE status=? AND type=? AND expires_at>?;
	`)
	_, err = tx.Tx.ExecContext(ctx, statement, offers2.Done, currentTime, o.Status, o.Type, currentTime)
	if err != nil {
		return nil, offers2.Err.Wrap(errs.Combine(err, tx.Rollback()))
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
		dbx.Offer_Type(int(o.Type)),
	)
	if err != nil {
		return nil, offers2.Err.Wrap(errs.Combine(err, tx.Rollback()))
	}

	newOffer, err := convertDBOffer(offerDbx)
	if err != nil {
		return nil, offers2.Err.Wrap(errs.Combine(err, tx.Rollback()))
	}

	return newOffer, offers2.Err.Wrap(tx.Commit())
}

// Redeem adds 1 to the amount of offersDB redeemed based on offer id
func (offers *offersDB) Redeem(ctx context.Context, oID int) error {
	statement := offers.db.Rebind(
		`UPDATE offersDB SET num_redeemed = num_redeemed + 1 where id = ? AND status = ? AND num_redeemed < redeemable_cap`,
	)

	_, err := offers.db.DB.ExecContext(ctx, statement, oID, offers2.Active)
	if err != nil {
		return offers2.Err.Wrap(err)
	}

	return nil
}

// Finish changes the offer status to be Done and its expiration date to be now based on offer id
func (offers *offersDB) Finish(ctx context.Context, oID int) error {
	updateFields := dbx.Offer_Update_Fields{
		Status:    dbx.Offer_Status(int(offers2.Done)),
		ExpiresAt: dbx.Offer_ExpiresAt(time.Now().UTC()),
	}

	offerID := dbx.Offer_Id(oID)

	_, err := offers.db.Update_Offer_By_Id(ctx, offerID, updateFields)
	if err != nil {
		return offers2.Err.Wrap(err)
	}

	return nil
}

func offersFromDBX(offersDbx []*dbx.Offer) ([]offers2.Offer, error) {
	var offers []offers2.Offer
	var errList errs.Group

	for _, offerDbx := range offersDbx {

		offer, err := convertDBOffer(offerDbx)
		if err != nil {
			errList.Add(err)
			continue
		}
		offers = append(offers, *offer)
	}

	return offers, errList.Err()
}

func convertDBOffer(offerDbx *dbx.Offer) (*offers2.Offer, error) {
	if offerDbx == nil {
		return nil, offers2.Err.New("offerDbx parameter is nil")
	}

	o := offers2.Offer{
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
		Status:                    offers2.OfferStatus(offerDbx.Status),
		Type:                      offers2.OfferType(offerDbx.Type),
	}

	return &o, nil
}
