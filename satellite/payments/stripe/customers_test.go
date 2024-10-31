// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/stripe"
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

		userIDs := []uuid.UUID{}
		for i := 0; i < custLen*2+3; i++ {
			userID, err := uuid.New()
			require.NoError(t, err)
			userIDs = append(userIDs, userID)
		}

		// order ids to be able to compare results easily
		sort.Slice(userIDs, func(i, j int) bool {
			return userIDs[i].Less(userIDs[j])
		})

		for i, userID := range userIDs {
			cus := stripe.Customer{
				ID:     "customerID" + strconv.Itoa(i),
				UserID: userID,
			}

			err := customersDB.Insert(ctx, cus.UserID, cus.ID)
			require.NoError(t, err)

			// Ensure that every insert gets a different "created at" time.
			waitForTimeToChange()
		}

		page, err := customersDB.List(ctx, uuid.UUID{}, custLen, time.Now())
		require.NoError(t, err)
		require.Equal(t, custLen, len(page.Customers))

		require.True(t, page.Next)
		require.Equal(t, userIDs[4], page.Cursor)

		for i, cus := range page.Customers {
			require.Equal(t, "customerID"+strconv.Itoa(i), cus.ID)
		}

		page, err = customersDB.List(ctx, page.Cursor, custLen, time.Now())
		require.NoError(t, err)
		require.Equal(t, custLen, len(page.Customers))

		require.True(t, page.Next)
		require.Equal(t, userIDs[9], page.Cursor)

		for i, cus := range page.Customers {
			require.Equal(t, "customerID"+strconv.Itoa(i+custLen), cus.ID)
		}

		page, err = customersDB.List(ctx, page.Cursor, custLen, time.Now())
		require.NoError(t, err)
		require.Equal(t, 3, len(page.Customers))

		require.False(t, page.Next)
		require.True(t, page.Cursor.IsZero())

		for i, cus := range page.Customers {
			require.Equal(t, "customerID"+strconv.Itoa(i+(2*custLen)), cus.ID)
		}
	})
}

func waitForTimeToChange() {
	t := time.Now()
	for time.Since(t) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
}
