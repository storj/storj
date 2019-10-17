// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestCustomersRepository(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		customers := db.Customers()

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
