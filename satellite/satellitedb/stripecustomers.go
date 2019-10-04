package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// accounts is an implementation of payments.Accounts
type stripe_customers struct {
	db *dbx.DB
}

func (accounts *stripe_customers) Insert(ctx context.Context, userID uuid.UUID, customerID string) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = accounts.db.Create_StripeCustomers(
		ctx,
		dbx.StripeCustomers_UserId(userID[:]),
		dbx.StripeCustomers_CustomerId(customerID),
	)

	return err
}
