// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/cockroachdb/cockroach-go/crdb"
	"github.com/zeebo/errs"

	"storj.io/storj/private/currency"
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

// GetCurrent returns offers that has not expired based on offer type
func (db *offersDB) GetActiveOffersByType(ctx context.Context, offerType rewards.OfferType) (rewards.Offers, error) {
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

	rows, err := db.db.DB.QueryContext(ctx, db.db.Rebind(statement), rewards.Active, offerType, time.Now().UTC(), offerType, rewards.Default)
	if err != nil {
		return nil, rewards.ErrOfferNotExist.Wrap(err)
	}

	var (
		awardCreditInCents        int
		inviteeCreditInCents      int
		awardCreditDurationDays   sql.NullInt64
		inviteeCreditDurationDays sql.NullInt64
		redeemableCap             sql.NullInt64
	)

	defer func() { err = errs.Combine(err, rows.Close()) }()
	results := rewards.Offers{}
	for rows.Next() {
		o := rewards.Offer{}
		err := rows.Scan(&o.ID, &o.Name, &o.Description, &awardCreditInCents, &inviteeCreditInCents, &awardCreditDurationDays, &inviteeCreditDurationDays, &redeemableCap, &o.ExpiresAt, &o.CreatedAt, &o.Status, &o.Type)
		if err != nil {
			return results, Error.Wrap(err)
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
		o.ExpiresAt = o.ExpiresAt.UTC()
		o.CreatedAt = o.CreatedAt.UTC()

		results = append(results, o)
	}

	if len(results) < 1 {
		return results, rewards.ErrOfferNotExist.New("offerType: %d", offerType)
	}
	return results, nil
}

// Create inserts a new offer into the db
func (db *offersDB) Create(ctx context.Context, o *rewards.NewOffer) (*rewards.Offer, error) {
	currentTime := time.Now().UTC()
	if o.ExpiresAt.Before(currentTime) {
		return nil, offerErr.New("expiration time: %v can't be before: %v", o.ExpiresAt, currentTime)
	}

	if o.Status == rewards.Default {
		o.ExpiresAt = time.Now().UTC().AddDate(100, 0, 0)
	}

	var id int64

	err := crdb.ExecuteTx(ctx, db.db.DB, nil, func(tx *sql.Tx) error {
		// If there's an existing current offer, update its status to Done and set its expires_at to be NOW()
		switch o.Type {
		case rewards.Partner:
			statement := `
				UPDATE offers SET status=?, expires_at=?
				WHERE status=? AND type=? AND expires_at>? AND name=?;`
			_, err := tx.ExecContext(ctx, db.db.Rebind(statement), rewards.Done, currentTime, o.Status, o.Type, currentTime, o.Name)
			if err != nil {
				return offerErr.Wrap(err)
			}

		default:
			statement := `
				UPDATE offers SET status=?, expires_at=?
				WHERE status=? AND type=? AND expires_at>?;`
			_, err := tx.ExecContext(ctx, db.db.Rebind(statement), rewards.Done, currentTime, o.Status, o.Type, currentTime)
			if err != nil {
				return offerErr.Wrap(err)
			}
		}
		statement := `
			INSERT INTO offers (name, description, award_credit_in_cents, invitee_credit_in_cents, award_credit_duration_days, 
				invitee_credit_duration_days, redeemable_cap, expires_at, created_at, status, type)
					VALUES (?::TEXT, ?::TEXT, ?::INT, ?::INT, ?::INT, ?::INT, ?::INT, ?::timestamptz, ?::timestamptz, ?::INT, ?::INT)
						RETURNING id;
		`
		row := tx.QueryRowContext(ctx, db.db.Rebind(statement),
			o.Name,
			o.Description,
			o.AwardCredit.Cents(),
			o.InviteeCredit.Cents(),
			o.AwardCreditDurationDays,
			o.InviteeCreditDurationDays,
			o.RedeemableCap,
			o.ExpiresAt,
			currentTime,
			o.Status,
			o.Type,
		)

		return row.Scan(&id)
	})

	return &rewards.Offer{
		ID:                        int(id),
		Name:                      o.Name,
		Description:               o.Description,
		AwardCredit:               o.AwardCredit,
		InviteeCredit:             o.InviteeCredit,
		AwardCreditDurationDays:   o.AwardCreditDurationDays,
		InviteeCreditDurationDays: o.InviteeCreditDurationDays,
		RedeemableCap:             o.RedeemableCap,
		ExpiresAt:                 o.ExpiresAt,
		CreatedAt:                 currentTime,
		Status:                    o.Status,
		Type:                      o.Type,
	}, offerErr.Wrap(err)
}

// Finish changes the offer status to be Done and its expiration date to be now based on offer id
func (db *offersDB) Finish(ctx context.Context, oID int) error {
	return offerErr.Wrap(
		db.db.UpdateNoReturn_Offer_By_Id(ctx,
			dbx.Offer_Id(oID), dbx.Offer_Update_Fields{
				Status:    dbx.Offer_Status(int(rewards.Done)),
				ExpiresAt: dbx.Offer_ExpiresAt(time.Now().UTC()),
			}))
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
		ExpiresAt:                 offerDbx.ExpiresAt.UTC(),
		AwardCreditDurationDays:   awardCreditDurationDays,
		InviteeCreditDurationDays: inviteeCreditDurationDays,
		CreatedAt:                 offerDbx.CreatedAt.UTC(),
		Status:                    rewards.OfferStatus(offerDbx.Status),
		Type:                      rewards.OfferType(offerDbx.Type),
	}

	return &o, nil
}
