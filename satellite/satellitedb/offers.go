// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/currency"
	"storj.io/storj/satellite/rewards"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

var (
	// offerErr is the default offer errors class
	offerErr = errs.Class("offers error")
)

type offersDB struct {
	db *dbx.DB
}

// ListAll returns all offersDB from the db
func (db *offersDB) ListAll(ctx context.Context) (rewards.Offers, error) {
	offersDbx, err := db.db.All_Offer_OrderBy_Asc_Id(ctx)
	if err != nil {
		return nil, offerErr.Wrap(err)
	}

	return offersFromDBX(offersDbx)
}

// GetCurrent returns an offer that has not expired based on offer type
func (db *offersDB) GetCurrentByType(ctx context.Context, offerType rewards.OfferType) (*rewards.Offer, error) {
	var statement string
	const columns = "id, name, description, award_credit_in_cents, invitee_credit_in_cents, award_credit_duration_days, invitee_credit_duration_days, redeemable_cap, expires_at, created_at, status, type"
	statement = `
		WITH o AS (
			SELECT ` + columns + ` FROM offers WHERE status=? AND type=? AND expires_at>? 
		)
		SELECT ` + columns + ` FROM o
		UNION ALL
		SELECT ` + columns + ` FROM offers
		WHERE type=? AND status=?
		AND NOT EXISTS (
			SELECT id FROM o
		) order by created_at desc;`

	rows := db.db.DB.QueryRowContext(ctx, db.db.Rebind(statement), rewards.Active, offerType, time.Now().UTC(), offerType, rewards.Default)

	var (
		awardCreditInCents        int
		inviteeCreditInCents      int
		awardCreditDurationDays   sql.NullInt64
		inviteeCreditDurationDays sql.NullInt64
		redeemableCap             sql.NullInt64
	)

	o := rewards.Offer{}
	err := rows.Scan(&o.ID, &o.Name, &o.Description, &awardCreditInCents, &inviteeCreditInCents, &awardCreditDurationDays, &inviteeCreditDurationDays, &redeemableCap, &o.ExpiresAt, &o.CreatedAt, &o.Status, &o.Type)
	if err == sql.ErrNoRows {
		return nil, offerErr.New("no current offer")
	}
	if err != nil {
		return nil, offerErr.Wrap(err)
	}
	o.AwardCredit = currency.Cents(awardCreditInCents)
	o.InviteeCredit = currency.Cents(inviteeCreditInCents)
	if redeemableCap.Valid {
		o.RedeemableCap = int(redeemableCap.Int64)
	}
	if awardCreditDurationDays.Valid {
		o.AwardCreditDurationDays = int(awardCreditDurationDays.Int64)
	}
	if inviteeCreditDurationDays.Valid {
		o.InviteeCreditDurationDays = int(inviteeCreditDurationDays.Int64)
	}

	return &o, nil
}

// Create inserts a new offer into the db
func (db *offersDB) Create(ctx context.Context, o *rewards.NewOffer) (*rewards.Offer, error) {
	currentTime := time.Now()
	if o.ExpiresAt.Before(currentTime) {
		return nil, offerErr.New("expiration time: %v can't be before: %v", o.ExpiresAt, currentTime)
	}

	if o.Status == rewards.Default {
		o.ExpiresAt = time.Now().UTC().AddDate(100, 0, 0)
		o.RedeemableCap = 1
	}

	tx, err := db.db.Open(ctx)
	if err != nil {
		return nil, offerErr.Wrap(err)
	}

	// If there's an existing current offer, update its status to Done and set its expires_at to be NOW()
	statement := db.db.Rebind(`
		UPDATE offers SET status=?, expires_at=?
		WHERE status=? AND type=? AND expires_at>?;
	`)
	_, err = tx.Tx.ExecContext(ctx, statement, rewards.Done, currentTime, o.Status, o.Type, currentTime)
	if err != nil {
		return nil, offerErr.Wrap(errs.Combine(err, tx.Rollback()))
	}

	offerDbx, err := tx.Create_Offer(ctx,
		dbx.Offer_Name(o.Name),
		dbx.Offer_Description(o.Description),
		dbx.Offer_AwardCreditInCents(o.AwardCredit.Cents()),
		dbx.Offer_InviteeCreditInCents(o.InviteeCredit.Cents()),
		dbx.Offer_ExpiresAt(o.ExpiresAt),
		dbx.Offer_Status(int(o.Status)),
		dbx.Offer_Type(int(o.Type)),
		dbx.Offer_Create_Fields{
			AwardCreditDurationDays:   dbx.Offer_AwardCreditDurationDays(o.AwardCreditDurationDays),
			InviteeCreditDurationDays: dbx.Offer_InviteeCreditDurationDays(o.InviteeCreditDurationDays),
			RedeemableCap:             dbx.Offer_RedeemableCap(o.RedeemableCap),
		},
	)
	if err != nil {
		return nil, offerErr.Wrap(errs.Combine(err, tx.Rollback()))
	}

	newOffer, err := convertDBOffer(offerDbx)
	if err != nil {
		return nil, offerErr.Wrap(errs.Combine(err, tx.Rollback()))
	}

	return newOffer, offerErr.Wrap(tx.Commit())
}

// Finish changes the offer status to be Done and its expiration date to be now based on offer id
func (db *offersDB) Finish(ctx context.Context, oID int) error {
	updateFields := dbx.Offer_Update_Fields{
		Status:    dbx.Offer_Status(int(rewards.Done)),
		ExpiresAt: dbx.Offer_ExpiresAt(time.Now().UTC()),
	}

	offerID := dbx.Offer_Id(oID)

	_, err := db.db.Update_Offer_By_Id(ctx, offerID, updateFields)
	if err != nil {
		return offerErr.Wrap(err)
	}

	return nil
}

func offersFromDBX(offersDbx []*dbx.Offer) (rewards.Offers, error) {
	var offers []rewards.Offer
	errList := new(errs.Group)

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

func convertDBOffer(offerDbx *dbx.Offer) (*rewards.Offer, error) {
	if offerDbx == nil {
		return nil, offerErr.New("offerDbx parameter is nil")
	}

	var redeemableCap, awardCreditDurationDays, inviteeCreditDurationDays int
	if offerDbx.RedeemableCap != nil {
		redeemableCap = *offerDbx.RedeemableCap
	}
	if offerDbx.AwardCreditDurationDays != nil {
		awardCreditDurationDays = *offerDbx.AwardCreditDurationDays
	}
	if offerDbx.InviteeCreditDurationDays != nil {
		inviteeCreditDurationDays = *offerDbx.InviteeCreditDurationDays
	}

	o := rewards.Offer{
		ID:                        offerDbx.Id,
		Name:                      offerDbx.Name,
		Description:               offerDbx.Description,
		AwardCredit:               currency.Cents(offerDbx.AwardCreditInCents),
		InviteeCredit:             currency.Cents(offerDbx.InviteeCreditInCents),
		RedeemableCap:             redeemableCap,
		ExpiresAt:                 offerDbx.ExpiresAt,
		AwardCreditDurationDays:   awardCreditDurationDays,
		InviteeCreditDurationDays: inviteeCreditDurationDays,
		CreatedAt:                 offerDbx.CreatedAt,
		Status:                    rewards.OfferStatus(offerDbx.Status),
		Type:                      rewards.OfferType(offerDbx.Type),
	}

	return &o, nil
}
