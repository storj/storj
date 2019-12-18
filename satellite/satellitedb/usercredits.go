// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/private/currency"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/rewards"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that usercredits implements console.UserCredits.
var _ console.UserCredits = (*usercredits)(nil)

type usercredits struct {
	db *satelliteDB
	tx *dbx.Tx
}

// GetCreditUsage returns the total amount of referral a user has made based on user id, total available credits, and total used credits based on user id
func (c *usercredits) GetCreditUsage(ctx context.Context, userID uuid.UUID, expirationEndDate time.Time) (*console.UserCreditUsage, error) {
	usageRows, err := c.db.DB.QueryContext(ctx, c.db.Rebind(`SELECT a.used_credit, b.available_credit, c.referred
		FROM (SELECT SUM(credits_used_in_cents) AS used_credit FROM user_credits WHERE user_id = ?) AS a,
		(SELECT SUM(credits_earned_in_cents - credits_used_in_cents) AS available_credit FROM user_credits WHERE expires_at > ? AND user_id = ?) AS b,
		(SELECT count(id) AS referred FROM user_credits WHERE user_credits.user_id = ? AND user_credits.type = ?) AS c;`), userID[:], expirationEndDate, userID[:], userID[:], console.Referrer)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, usageRows.Close()) }()

	usage := console.UserCreditUsage{}

	for usageRows.Next() {

		var (
			usedCreditInCents      sql.NullInt64
			availableCreditInCents sql.NullInt64
			referred               sql.NullInt64
		)
		err = usageRows.Scan(&usedCreditInCents, &availableCreditInCents, &referred)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		usage.Referred += referred.Int64
		usage.UsedCredits = usage.UsedCredits.Add(currency.Cents(int(usedCreditInCents.Int64)))
		usage.AvailableCredits = usage.AvailableCredits.Add(currency.Cents(int(availableCreditInCents.Int64)))
	}

	return &usage, nil
}

// Create insert a new record of user credit
func (c *usercredits) Create(ctx context.Context, userCredit console.CreateCredit) (err error) {
	if userCredit.ExpiresAt.Before(time.Now().UTC()) {
		return errs.New("user credit is already expired")
	}

	var referrerID []byte
	if userCredit.ReferredBy != nil {
		referrerID = userCredit.ReferredBy[:]
	}

	var shouldCreate bool
	switch userCredit.OfferInfo.Type {
	case rewards.Partner:
		shouldCreate = false
	default:
		shouldCreate = userCredit.OfferInfo.Status.IsDefault()
	}

	var dbExec interface {
		ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	}

	if c.tx != nil {
		dbExec = c.tx.Tx
	} else {
		dbExec = c.db.DB
	}

	var (
		result    sql.Result
		statement string
	)
	statement = `
			INSERT INTO user_credits (user_id, offer_id, credits_earned_in_cents, credits_used_in_cents, expires_at, referred_by, type, created_at)
				SELECT * FROM (VALUES (?::bytea, ?::int, ?::int, 0, ?::timestamp, NULLIF(?::bytea, ?::bytea), ?::text, now())) AS v
					WHERE COALESCE((SELECT COUNT(offer_id) FROM user_credits WHERE offer_id = ? AND referred_by IS NOT NULL ) < NULLIF(?, 0), ?);
		`
	result, err = dbExec.ExecContext(ctx, c.db.Rebind(statement),
		userCredit.UserID[:],
		userCredit.OfferID,
		userCredit.CreditsEarned.Cents(),
		userCredit.ExpiresAt, referrerID, new([]byte),
		userCredit.Type,
		userCredit.OfferID,
		userCredit.OfferInfo.RedeemableCap, shouldCreate)

	if err != nil {
		// check to see if there's a constraint error
		if pgutil.IsConstraintError(err) {
			_, err := dbExec.ExecContext(ctx, c.db.Rebind(`UPDATE offers SET status = ? AND expires_at = ? WHERE id = ?`), rewards.Done, time.Now().UTC(), userCredit.OfferID)
			if err != nil {
				return errs.Wrap(err)
			}

			return rewards.ErrReachedMaxCapacity.Wrap(err)
		}

		return errs.Wrap(err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errs.Wrap(err)
	}

	if rows != 1 {
		return rewards.ErrReachedMaxCapacity.New("failed to create new credit")
	}

	return nil
}

// UpdateEarnedCredits updates user credits after user activated their account
func (c *usercredits) UpdateEarnedCredits(ctx context.Context, userID uuid.UUID) error {
	statement := `
		UPDATE user_credits SET credits_earned_in_cents = offers.invitee_credit_in_cents
			FROM offers
			WHERE user_id = ? AND credits_earned_in_cents = 0 AND offer_id = offers.id
	`

	result, err := c.db.DB.ExecContext(ctx, c.db.Rebind(statement), userID[:])
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return console.NoCreditForUpdateErr.New("row affected: %d", affected)
	}

	return nil
}

// UpdateAvailableCredits updates user's available credits based on their spending and the time of their spending
func (c *usercredits) UpdateAvailableCredits(ctx context.Context, creditsToCharge int, id uuid.UUID, expirationEndDate time.Time) (remainingCharge int, err error) {
	tx, err := c.db.Open(ctx)
	if err != nil {
		return creditsToCharge, errs.Wrap(err)
	}

	availableCredits, err := tx.All_UserCredit_By_UserId_And_ExpiresAt_Greater_And_CreditsUsedInCents_Less_CreditsEarnedInCents_OrderBy_Asc_ExpiresAt(ctx,
		dbx.UserCredit_UserId(id[:]),
		dbx.UserCredit_ExpiresAt(expirationEndDate),
	)
	if err != nil {
		return creditsToCharge, errs.Wrap(errs.Combine(err, tx.Rollback()))
	}
	if len(availableCredits) == 0 {
		return creditsToCharge, errs.Combine(errs.New("No available credits"), tx.Commit())
	}

	values := make([]interface{}, len(availableCredits)*2)
	rowIds := make([]interface{}, len(availableCredits))

	remainingCharge = creditsToCharge
	for i, credit := range availableCredits {
		if remainingCharge == 0 {
			break
		}

		creditsForUpdateInCents := credit.CreditsEarnedInCents - credit.CreditsUsedInCents

		if remainingCharge < creditsForUpdateInCents {
			creditsForUpdateInCents = remainingCharge
		}

		values[i%2] = credit.Id
		values[(i%2 + 1)] = creditsForUpdateInCents
		rowIds[i] = credit.Id

		remainingCharge -= creditsForUpdateInCents
	}

	values = append(values, rowIds...)

	statement := generateQuery(len(availableCredits), true)

	_, err = tx.Tx.ExecContext(ctx, c.db.Rebind(`UPDATE user_credits SET credits_used_in_cents = CASE `+statement), values...)
	if err != nil {
		return creditsToCharge, errs.Wrap(errs.Combine(err, tx.Rollback()))
	}
	return remainingCharge, errs.Wrap(tx.Commit())
}

func generateQuery(totalRows int, toInt bool) (query string) {
	whereClause := `WHERE id IN (`
	condition := `WHEN id=? THEN ? `
	if toInt {
		condition = `WHEN id=? THEN ?::int `
	}

	for i := 0; i < totalRows; i++ {
		query += condition

		if i == totalRows-1 {
			query += ` END ` + whereClause + ` ?);`
			break
		}
		whereClause += `?, `
	}

	return query
}
