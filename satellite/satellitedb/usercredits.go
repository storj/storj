// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"strconv"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type usercredits struct {
	db *dbx.DB
}

type updateInfo struct {
	id      int
	credits int
}

// TotalReferredCount returns the total amount of referral a user has made based on user id
func (c *usercredits) TotalReferredCount(ctx context.Context, id uuid.UUID) (int64, error) {
	totalReferred, err := c.db.Count_UserCredit_By_ReferredBy(ctx, dbx.UserCredit_ReferredBy(id[:]))
	if err != nil {
		return totalReferred, errs.Wrap(err)
	}

	return totalReferred, nil
}

// GetAvailableCredits returns all records of user credit that are not expired or used
func (c *usercredits) GetAvailableCredits(ctx context.Context, referrerID uuid.UUID, expirationEndDate time.Time) ([]console.UserCredit, error) {
	availableCredits, err := c.db.All_UserCredit_By_UserId_And_ExpiresAt_Greater_And_CreditsUsedInCents_Less_CreditsEarnedInCents_OrderBy_Asc_ExpiresAt(ctx,
		dbx.UserCredit_UserId(referrerID[:]),
		dbx.UserCredit_ExpiresAt(expirationEndDate),
	)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return fromDBX(availableCredits)
}

// Create insert a new record of user credit
func (c *usercredits) Create(ctx context.Context, userCredit console.UserCredit) (*console.UserCredit, error) {
	createdUser, err := c.db.Create_UserCredit(ctx,
		dbx.UserCredit_UserId(userCredit.UserID[:]),
		dbx.UserCredit_OfferId(userCredit.OfferID),
		dbx.UserCredit_CreditsEarnedInCents(userCredit.CreditsEarnedInCents),
		dbx.UserCredit_ExpiresAt(userCredit.ExpiresAt),
		dbx.UserCredit_Create_Fields{
			ReferredBy: dbx.UserCredit_ReferredBy(userCredit.ReferredBy[:]),
		},
	)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return convertDBX(createdUser)
}

// UpdateAvailableCredits updates user's available credits based on their spending and the time of their spending
func (c *usercredits) UpdateAvailableCredits(ctx context.Context, chargedCredits int, id uuid.UUID, expirationEndDate time.Time) (remainingCharge int, err error) {
	tx, err := c.db.Open(ctx)
	if err != nil {
		return chargedCredits, errs.Wrap(err)
	}

	availableCredits, err := tx.All_UserCredit_By_UserId_And_ExpiresAt_Greater_And_CreditsUsedInCents_Less_CreditsEarnedInCents_OrderBy_Asc_ExpiresAt(ctx,
		dbx.UserCredit_UserId(id[:]),
		dbx.UserCredit_ExpiresAt(expirationEndDate),
	)
	if err != nil {
		return chargedCredits, errs.Wrap(errs.Combine(err, tx.Rollback()))
	}
	if len(availableCredits) == 0 {
		return chargedCredits, errs.Combine(errs.New("No available credits"), tx.Commit())
	}

	var infos []updateInfo
	var creditsToCharge = chargedCredits
	for _, credit := range availableCredits {
		if creditsToCharge == 0 {
			break
		}

		creditsForUpdate := credit.CreditsEarnedInCents - credit.CreditsUsedInCents

		if creditsToCharge < creditsForUpdate {
			creditsForUpdate = creditsToCharge
		}

		infos = append(infos, updateInfo{
			id:      credit.Id,
			credits: creditsForUpdate,
		})

		creditsToCharge -= creditsForUpdate
	}

	statement := `UPDATE user_credits SET
		credits_used_in_cents = CASE id ` + convertToSQLFormat(infos)

	_, err = tx.Tx.ExecContext(ctx, c.db.Rebind(statement))
	if err != nil {
		return chargedCredits, errs.Wrap(errs.Combine(err, tx.Rollback()))
	}
	return creditsToCharge, errs.Wrap(tx.Commit())
}

func convertToSQLFormat(updateInfo []updateInfo) (updateStr string) {
	for i, info := range updateInfo {
		updateStr += `WHEN ` + strconv.Itoa(info.id) + ` THEN ` + strconv.Itoa(info.credits)
		if i == len(updateInfo)-1 {
			updateStr += ` END;`
			break
		}
		updateStr += ` `
	}

	return updateStr
}

func fromDBX(userCreditsDBX []*dbx.UserCredit) ([]console.UserCredit, error) {
	var userCredits []console.UserCredit
	errList := new(errs.Group)

	for _, userCredit := range userCreditsDBX {

		uc, err := convertDBX(userCredit)
		if err != nil {
			errList.Add(err)
			continue
		}
		userCredits = append(userCredits, *uc)
	}

	return userCredits, errList.Err()
}

func convertDBX(userCreditDBX *dbx.UserCredit) (*console.UserCredit, error) {
	if userCreditDBX == nil {
		return nil, errs.New("userCreditDBX parameter is nil")
	}

	userID, err := bytesToUUID(userCreditDBX.UserId)
	if err != nil {
		return nil, err
	}

	referredByID, err := bytesToUUID(userCreditDBX.ReferredBy)
	if err != nil {
		return nil, err
	}

	return &console.UserCredit{
		ID:                   userCreditDBX.Id,
		UserID:               userID,
		OfferID:              userCreditDBX.OfferId,
		ReferredBy:           referredByID,
		CreditsEarnedInCents: userCreditDBX.CreditsEarnedInCents,
		CreditsUsedInCents:   userCreditDBX.CreditsUsedInCents,
		ExpiresAt:            userCreditDBX.ExpiresAt,
		CreatedAt:            userCreditDBX.CreatedAt,
	}, nil
}
