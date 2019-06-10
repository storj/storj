package satellitedb

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"github.com/skyrings/skyring-common/tools/uuid"

	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type usercredits struct {
	db dbx.Methods
}

func (c *usercredits) TotalReferredCountByUserID(ctx context.Context, id uuid.UUID) (int64, error) {
	totalReferred, err := c.db.Count_UserCredit_By_ReferredBy(ctx, dbx.UserCredit_ReferredBy(id[:]))
	if err != nil {
		return totalReferred, errs.Wrap(err)
	}

	return totalReferred, nil
}

func (c *usercredits) AvailableCreditsByUserID(ctx context.Context, id uuid.UUID, expirationEndDate time.Time) ([]Console.UserCredit, error) {
	availableCredits, err := c.db.All_UserCredit_By_UserId_And_ExpiresAt_Greater_And_CreditsUsedInCents_Less_CreditsEarnedInCents(ctx,
		dbx.UserCredit_UserId(id[:]),
		dbx.UserCredit_ExpiresAt(expirationEndDate),
	)
	if err != nil {
		return availableCredits, errs.Wrap(err)
	}

	return availableCredits, nil
}

//func (c *usercredits) Update
