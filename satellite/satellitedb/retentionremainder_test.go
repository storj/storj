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
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestRetentionRemainderCharges(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		retentionRemainderDB := db.RetentionRemainderCharges()

		projectID := testrand.UUID()
		bucketName := "test-bucket"
		now := time.Now()
		now = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

		charge1 := accounting.RetentionRemainderCharge{
			ProjectID:          projectID,
			BucketName:         bucketName,
			DeletedAt:          now,
			RemainderByteHours: 360.0,
			ProductID:          1,
		}
		require.NoError(t, retentionRemainderDB.Upsert(ctx, charge1))

		charge2 := accounting.RetentionRemainderCharge{
			ProjectID:          projectID,
			BucketName:         bucketName,
			DeletedAt:          now.AddDate(0, 1, 0),
			RemainderByteHours: 480.0,
			ProductID:          1,
		}
		require.NoError(t, retentionRemainderDB.Upsert(ctx, charge2))

		otherProjectID := testrand.UUID()
		charge3 := accounting.RetentionRemainderCharge{
			ProjectID:          otherProjectID,
			BucketName:         "other-bucket",
			DeletedAt:          now,
			RemainderByteHours: 600.0,
			ProductID:          2,
		}
		require.NoError(t, retentionRemainderDB.Upsert(ctx, charge3))

		charge4 := accounting.RetentionRemainderCharge{
			ProjectID:          projectID,
			BucketName:         bucketName,
			DeletedAt:          now.AddDate(0, 2, 0),
			RemainderByteHours: 360.0,
			ProductID:          1,
			Billed:             true,
		}
		require.NoError(t, retentionRemainderDB.Upsert(ctx, charge4))

		// Test GetUnbilledCharges - should only get unbilled charges for the project in the time range
		from := now
		to := now.AddDate(0, 2, 0)

		options := accounting.GetUnbilledChargesOptions{
			ProjectID: projectID,
			From:      from,
			To:        to,
			Limit:     1,
		}
		charges, token, err := retentionRemainderDB.GetUnbilledCharges(ctx, options)
		require.NoError(t, err)
		require.Len(t, charges, 1, "should get exactly 1 unbilled charge")
		firstCharge := charges[0]
		options.NextToken = token

		charges, _, err = retentionRemainderDB.GetUnbilledCharges(ctx, options)
		require.NoError(t, err)
		require.Len(t, charges, 1, "should get exactly 1 unbilled charge")
		require.NotEqual(t, firstCharge, charges[0])

		options.NextToken = nil
		options.Limit = 5
		charges, _, err = retentionRemainderDB.GetUnbilledCharges(ctx, options)
		require.NoError(t, err)
		require.Len(t, charges, 2, "should get exactly 2 unbilled charges")

		// Verify the charges
		totalRemainderByteHours := 0.0
		for _, charge := range charges {
			require.Equal(t, projectID, charge.ProjectID)
			require.False(t, charge.Billed)
			totalRemainderByteHours += charge.RemainderByteHours
		}

		expectedTotal := charge1.RemainderByteHours + charge2.RemainderByteHours
		require.InDelta(t, expectedTotal, totalRemainderByteHours, 0.1, "total byte-hours should match")

		// Test GetUnbilledCharges with different time range - should get none
		earlyFrom := now.AddDate(-1, 0, 0)
		earlyTo := now.AddDate(-1, 2, 0)

		options.From = earlyFrom
		options.To = earlyTo

		earlyCharges, _, err := retentionRemainderDB.GetUnbilledCharges(ctx, options)
		require.NoError(t, err)
		require.Empty(t, earlyCharges, "should get no charges outside time range")

		// Test GetUnbilledCharges for other project
		options.ProjectID = otherProjectID
		options.From = from
		options.To = to
		otherCharges, _, err := retentionRemainderDB.GetUnbilledCharges(ctx, options)
		require.NoError(t, err)
		require.Len(t, otherCharges, 1, "should get 1 charge for other project")
		require.Equal(t, otherProjectID, otherCharges[0].ProjectID)
		require.Equal(t, int32(2), otherCharges[0].ProductID)

		// Test MarkChargesAsBilled
		err = retentionRemainderDB.MarkChargesAsBilled(ctx, projectID, from, to)
		require.NoError(t, err)

		options.ProjectID = projectID

		// Verify charges are now marked as billed
		chargesAfterBilling, _, err := retentionRemainderDB.GetUnbilledCharges(ctx, options)
		require.NoError(t, err)
		require.Len(t, chargesAfterBilling, 0, "should get no unbilled charges after marking as billed")

		options.ProjectID = otherProjectID

		// Verify other project's charges are still unbilled
		otherChargesStillUnbilled, _, err := retentionRemainderDB.GetUnbilledCharges(ctx, options)
		require.NoError(t, err)
		require.Len(t, otherChargesStillUnbilled, 1, "other project's charges should still be unbilled")

		// verify that upsert adds to existing charge
		additionalCharge := accounting.RetentionRemainderCharge{
			ProjectID:          otherProjectID,
			BucketName:         "other-bucket",
			DeletedAt:          charge3.DeletedAt,
			RemainderByteHours: 240.0,
			ProductID:          1,
		}
		require.NoError(t, retentionRemainderDB.Upsert(ctx, additionalCharge))
		chargesAfterUpsert, _, err := retentionRemainderDB.GetUnbilledCharges(ctx, options)
		require.NoError(t, err)
		require.Len(t, chargesAfterUpsert, 1, "should still be 1 charge after upsert")
		expectedTotal = charge3.RemainderByteHours + additionalCharge.RemainderByteHours
		require.InDelta(t, expectedTotal, chargesAfterUpsert[0].RemainderByteHours, 0.1, "total byte-hours should match after upsert")
	})
}
