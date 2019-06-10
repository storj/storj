package satellitedb

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type usercredits struct {
	db *dbx.DB
}

func (c *usercredits) TotalReferredCount(ctx context.Context, id uuid.UUID) (int64, error) {
	totalReferred, err := c.db.Count_UserCredit_By_ReferredBy(ctx, dbx.UserCredit_ReferredBy(id[:]))
	if err != nil {
		return totalReferred, errs.Wrap(err)
	}

	return totalReferred, nil
}

func (c *usercredits) AvailableCredits(ctx context.Context, id uuid.UUID, expirationEndDate time.Time) ([]console.UserCredit, error) {
	availableCredits, err := c.db.All_UserCredit_By_UserId_And_ExpiresAt_Greater_And_CreditsUsedInCents_Less_CreditsEarnedInCents_OrderBy_Asc_ExpiresAt(ctx,
		dbx.UserCredit_UserId(id[:]),
		dbx.UserCredit_ExpiresAt(expirationEndDate),
	)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return fromDBX(availableCredits)
}

func (c *usercredits) Create(ctx context.Context, userCredit console.UserCredit) error {
	_, err := c.db.Create_UserCredit(ctx,
		dbx.UserCredit_UserId(userCredit.UserID[:]),
		dbx.UserCredit_OfferId(userCredit.OfferID),
		dbx.UserCredit_CreditsEarnedInCents(userCredit.CreditsEarnedInCents),
		dbx.UserCredit_ExpiresAt(userCredit.ExpiresAt),
		dbx.UserCredit_Create_Fields{
			ReferredBy: dbx.UserCredit_ReferredBy(userCredit.ReferredBy[:]),
		},
	)
	if err != nil {
		return errs.Wrap(err)
	}

	return nil
}

func (c *usercredits) UpdateAvailableCredits(ctx context.Context, appliedCredits int, id uuid.UUID, expirationEndDate time.Time) error {
	tx, err := c.db.Open(ctx)
	if err != nil {
		return errs.Wrap(err)
	}

	availableCredits, err := tx.All_UserCredit_By_UserId_And_ExpiresAt_Greater_And_CreditsUsedInCents_Less_CreditsEarnedInCents_OrderBy_Asc_ExpiresAt(ctx,
		dbx.UserCredit_UserId(id[:]),
		dbx.UserCredit_ExpiresAt(expirationEndDate),
	)
	if err != nil {
		return errs.Wrap(errs.Combine(err, tx.Rollback()))
	}

	for _, credit := range availableCredits {
		if appliedCredits == 0 {
			break
		}

		updatedUsedCredit := credit.CreditsEarnedInCents

		if appliedCredits < credit.CreditsEarnedInCents {
			updatedUsedCredit = appliedCredits
		}

		_, err := tx.Tx.ExecContext(ctx, c.db.Rebind(`UPDATE user_credits SET credits_used_in_cents = ?`), updatedUsedCredit)
		if err != nil {
			return errs.Wrap(errs.Combine(err, tx.Rollback()))
		}
		appliedCredits = appliedCredits - (credit.CreditsEarnedInCents - credit.CreditsUsedInCents)
	}

	return errs.Wrap(tx.Commit())
}

func fromDBX(userCreditsDBX []*dbx.UserCredit) ([]console.UserCredit, error) {
	var userCredits []console.UserCredit
	var errList errs.Group

	for _, userCredit := range userCreditsDBX {

		uc, err := convertDBX(userCredit)
		if err != nil {
			errList.Add(err)
			continue
		}
		userCredits = append(userCredits, uc)
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

	credit := console.UserCredit{
		ID:                   userCreditDBX.Id,
		UserID:               userID,
		OfferID:              userCreditDBX.OfferId,
		ReferredBy:           referredByID,
		CreditsEarnedInCents: userCreditDBX.CreditsEarnedInCents,
		CreditsUsedInCents:   userCreditDBX.CreditsUsedInCents,
		ExpiresAt:            userCreditDBX.ExpiresAt,
		CreatedAt:            userCreditDBX.CreatedAt,
	}

	return &credit, nil
}
