// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestInvoiceLicenseRecords(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		records := db.StripeCoinPayments().LicenseRecords()

		start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
		userID := testrand.UUID()

		const productID = int32(7)

		full := stripe.CreateLicenseRecord{
			UserID:          userID,
			ProductID:       productID,
			Seats:           5,
			BilledDays:      31,
			DaysInPeriod:    31,
			UnitAmountCents: 2900,
		}
		// Same user and product, but a different prorated rate, so a separate record.
		prorated := stripe.CreateLicenseRecord{
			UserID:          userID,
			ProductID:       productID,
			Seats:           3,
			BilledDays:      11,
			DaysInPeriod:    31,
			UnitAmountCents: 1029,
		}

		t.Run("check reports nothing before anything is recorded", func(t *testing.T) {
			require.NoError(t, records.Check(ctx, userID, productID, full.BilledDays, start, end))
		})

		require.NoError(t, records.Create(ctx, []stripe.CreateLicenseRecord{full, prorated}, start, end))

		t.Run("check reports the recorded charges", func(t *testing.T) {
			require.ErrorIs(t, records.Check(ctx, userID, productID, full.BilledDays, start, end), stripe.ErrLicenseRecordExists)
			require.ErrorIs(t, records.Check(ctx, userID, productID, prorated.BilledDays, start, end), stripe.ErrLicenseRecordExists)

			// A different rate, product, user or period is a different charge.
			require.NoError(t, records.Check(ctx, userID, productID, 7, start, end))
			require.NoError(t, records.Check(ctx, userID, productID+1, full.BilledDays, start, end))
			require.NoError(t, records.Check(ctx, testrand.UUID(), productID, full.BilledDays, start, end))
			require.NoError(t, records.Check(ctx, userID, productID, full.BilledDays, start.AddDate(0, -1, 0), start))
		})

		t.Run("recording the same charge again is rejected", func(t *testing.T) {
			require.Error(t, records.Create(ctx, []stripe.CreateLicenseRecord{full}, start, end))
		})

		t.Run("a duplicate within one batch is rejected without writing any of it", func(t *testing.T) {
			otherUser := testrand.UUID()
			fresh := stripe.CreateLicenseRecord{
				UserID:          otherUser,
				ProductID:       productID,
				Seats:           1,
				BilledDays:      31,
				DaysInPeriod:    31,
				UnitAmountCents: 2900,
			}
			dup := fresh
			dup.Seats = 2

			err := records.Create(ctx, []stripe.CreateLicenseRecord{fresh, dup}, start, end)
			require.ErrorIs(t, err, stripe.ErrDuplicateLicenseRecord)

			// Nothing from the batch was written.
			written, err := records.ListUnappliedByUser(ctx, otherUser, start, end)
			require.NoError(t, err)
			require.Empty(t, written)

			// A differing product or rate in the same batch is fine.
			dup.ProductID = productID + 1
			require.NoError(t, records.Create(ctx, []stripe.CreateLicenseRecord{fresh, dup}, start, end))
			written, err = records.ListUnappliedByUser(ctx, otherUser, start, end)
			require.NoError(t, err)
			require.Len(t, written, 2)
		})

		var listed []stripe.LicenseRecord

		t.Run("list returns the unapplied records", func(t *testing.T) {
			var err error
			listed, err = records.ListUnappliedByUser(ctx, userID, start, end)
			require.NoError(t, err)
			require.Len(t, listed, 2)

			byBilledDays := map[int]stripe.LicenseRecord{}
			for _, record := range listed {
				byBilledDays[record.BilledDays] = record
			}

			got := byBilledDays[prorated.BilledDays]
			require.Equal(t, userID, got.UserID)
			require.Equal(t, productID, got.ProductID)
			require.EqualValues(t, 3, got.Seats)
			require.Equal(t, 31, got.DaysInPeriod)
			require.EqualValues(t, 1029, got.UnitAmountCents)
			require.Equal(t, start, got.PeriodStart.UTC())
			require.Equal(t, end, got.PeriodEnd.UTC())
			require.Zero(t, got.State)
			require.False(t, got.ID.IsZero())

			require.EqualValues(t, 5, byBilledDays[full.BilledDays].Seats)
		})

		t.Run("list is scoped to the user and period", func(t *testing.T) {
			other, err := records.ListUnappliedByUser(ctx, testrand.UUID(), start, end)
			require.NoError(t, err)
			require.Empty(t, other)

			otherPeriod, err := records.ListUnappliedByUser(ctx, userID, start.AddDate(0, -1, 0), start)
			require.NoError(t, err)
			require.Empty(t, otherPeriod)
		})

		t.Run("consumed records drop out of the listing", func(t *testing.T) {
			require.NoError(t, records.Consume(ctx, listed[0].ID))

			remaining, err := records.ListUnappliedByUser(ctx, userID, start, end)
			require.NoError(t, err)
			require.Len(t, remaining, 1)
			require.NotEqual(t, listed[0].ID, remaining[0].ID)

			// A consumed charge still counts as recorded, so it is never raised again.
			require.ErrorIs(t, records.Check(ctx, userID, productID, listed[0].BilledDays, start, end), stripe.ErrLicenseRecordExists)

			require.NoError(t, records.Consume(ctx, remaining[0].ID))
			empty, err := records.ListUnappliedByUser(ctx, userID, start, end)
			require.NoError(t, err)
			require.Empty(t, empty)
		})
	})
}
