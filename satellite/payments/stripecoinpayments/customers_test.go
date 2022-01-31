// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestCustomersRepository(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		customers := db.StripeCoinPayments().Customers()

		customerID := "customerID"
		userID, err := uuid.New()
		require.NoError(t, err)

		t.Run("Insert", func(t *testing.T) {
			err = customers.Insert(ctx, userID, customerID)
			assert.NoError(t, err)
		})

		t.Run("Can not insert duplicate customerID", func(t *testing.T) {
			err = customers.Insert(ctx, userID, customerID)
			assert.Error(t, err)
		})

		t.Run("GetCustomerID", func(t *testing.T) {
			id, err := customers.GetCustomerID(ctx, userID)
			assert.NoError(t, err)
			assert.Equal(t, id, customerID)
		})
	})
}

func TestCustomersRepositoryList(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		customersDB := db.StripeCoinPayments().Customers()

		const custLen = 5

		for i := 0; i < custLen*2+3; i++ {
			userID, err := uuid.New()
			require.NoError(t, err)

			cus := stripecoinpayments.Customer{
				ID:     "customerID" + strconv.Itoa(i),
				UserID: userID,
			}

			err = customersDB.Insert(ctx, cus.UserID, cus.ID)
			require.NoError(t, err)

			// Ensure that every insert gets a different "created at" time.
			waitForTimeToChange()
		}

		page, err := customersDB.List(ctx, 0, custLen, time.Now())
		require.NoError(t, err)
		require.Equal(t, custLen, len(page.Customers))

		assert.True(t, page.Next)
		assert.Equal(t, int64(5), page.NextOffset)

		for i, cus := range page.Customers {
			assert.Equal(t, "customerID"+strconv.Itoa(12-i), cus.ID)
		}

		page, err = customersDB.List(ctx, page.NextOffset, custLen, time.Now())
		require.NoError(t, err)
		require.Equal(t, custLen, len(page.Customers))

		assert.True(t, page.Next)
		assert.Equal(t, int64(10), page.NextOffset)

		for i, cus := range page.Customers {
			assert.Equal(t, "customerID"+strconv.Itoa(7-i), cus.ID)
		}

		page, err = customersDB.List(ctx, page.NextOffset, custLen, time.Now())
		require.NoError(t, err)
		require.Equal(t, 3, len(page.Customers))

		assert.False(t, page.Next)
		assert.Equal(t, int64(0), page.NextOffset)

		for i, cus := range page.Customers {
			assert.Equal(t, "customerID"+strconv.Itoa(2-i), cus.ID)

		}
	})
}

func waitForTimeToChange() {
	t := time.Now()
	for time.Since(t) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
}
