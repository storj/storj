package stripecoinpayments_test

import (
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestStripeCustomersRepository(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		stripecustomers := db.StripeCustomers()

		customerID := "customerID"
		userID, err := uuid.New()
		if err != nil {
			t.Fail()
		}

		t.Run("Insert", func(t *testing.T) {
			stripeCustomersIDs, err := stripecustomers.GetAllCustomerIDs(ctx)

			assert.NoError(t, err)
			assert.Equal(t, len(stripeCustomersIDs), 0)

			err = stripecustomers.Insert(ctx, *userID, customerID)
			assert.NoError(t, err)

			stripeCustomersIDs, err = stripecustomers.GetAllCustomerIDs(ctx)

			assert.NoError(t, err)
			assert.Equal(t, len(stripeCustomersIDs), 1)
		})
	})
}
