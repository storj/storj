// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestCustomersRepository(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		customers := db.StripeCoinPayments().Customers()

		customerID := "customerID"
		userID, err := uuid.New()
		require.NoError(t, err)

		t.Run("Insert", func(t *testing.T) {
			err = customers.Insert(ctx, *userID, customerID)
			assert.NoError(t, err)
		})

		t.Run("Can not insert duplicate customerID", func(t *testing.T) {
			err = customers.Insert(ctx, *userID, customerID)
			assert.Error(t, err)
		})

		t.Run("GetCustomerID", func(t *testing.T) {
			id, err := customers.GetCustomerID(ctx, *userID)
			assert.NoError(t, err)
			assert.Equal(t, id, customerID)
		})
	})
}

func TestCustomersRepositoryList(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		customersDB := db.StripeCoinPayments().Customers()

		const custLen = 5

		var customers []stripecoinpayments.Customer
		for i := 0; i < custLen; i++ {
			userID, err := uuid.New()
			require.NoError(t, err)

			cus := stripecoinpayments.Customer{
				ID:     "customerID" + strconv.Itoa(i),
				UserID: *userID,
			}

			err = customersDB.Insert(ctx, cus.UserID, cus.ID)
			require.NoError(t, err)

			customers = append(customers, cus)
		}

		page, err := customersDB.List(ctx, 0, custLen, time.Now())
		require.NoError(t, err)
		require.Equal(t, custLen, len(page.Customers))

		assert.False(t, page.Next)
		assert.Equal(t, int64(0), page.NextOffset)

		for _, cus1 := range page.Customers {
			for _, cus2 := range customers {
				if cus1.ID != cus2.ID {
					continue
				}

				assert.Equal(t, cus2.ID, cus1.ID)
				assert.Equal(t, cus2.UserID, cus1.UserID)
			}
		}
	})
}
