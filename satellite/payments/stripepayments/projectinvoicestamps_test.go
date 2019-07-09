// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripepayments_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments/stripepayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProjectInvoiceStamps(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		consoleDB := db.Console()
		stripeDB := db.StripePayments()

		startDate := time.Now().UTC()
		endDate := startDate.Add(time.Hour * 24)

		invoiceID := testrand.Bytes(8)

		//create project
		proj, err := consoleDB.Projects().Insert(ctx, &console.Project{
			Name: "test",
		})
		require.NoError(t, err)

		t.Run("create project invoice stamp", func(t *testing.T) {
			stamp, err := stripeDB.ProjectInvoiceStamps().Create(ctx, stripepayments.ProjectInvoiceStamp{
				ProjectID: proj.ID,
				InvoiceID: invoiceID,
				StartDate: startDate,
				EndDate:   endDate,
			})

			assert.NoError(t, err)
			assert.Equal(t, proj.ID, stamp.ProjectID)
			assert.Equal(t, invoiceID, stamp.InvoiceID)
			assert.Equal(t, startDate.Unix(), stamp.StartDate.Unix())
			assert.Equal(t, endDate.Unix(), stamp.EndDate.Unix())
		})

		t.Run("get by project id and start date", func(t *testing.T) {
			stamp, err := stripeDB.ProjectInvoiceStamps().GetByProjectIDStartDate(ctx, proj.ID, startDate)

			assert.NoError(t, err)
			assert.Equal(t, proj.ID, stamp.ProjectID)
			assert.Equal(t, invoiceID, stamp.InvoiceID)
			assert.Equal(t, startDate.Unix(), stamp.StartDate.Unix())
			assert.Equal(t, endDate.Unix(), stamp.EndDate.Unix())
		})
	})
}
