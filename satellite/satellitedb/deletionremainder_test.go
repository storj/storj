// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestDeletionRemainderCharges(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		deletionRemainderDB := db.DeletionRemainderCharges()

		projectID := testrand.UUID()
		bucketName := "test-bucket"
		now := time.Now()

		mbUint64 := func(mb int) uint64 {
			return uint64((memory.Size(mb) * memory.MB).Int64())
		}

		charge1 := accounting.DeletionRemainderCharge{
			ProjectID:      projectID,
			BucketName:     bucketName,
			CreatedAt:      now.Add(-30 * time.Hour),
			DeletedAt:      now.Add(-15 * time.Hour),
			ObjectSize:     mbUint64(100),
			RemainderHours: 360.0,
			ProductID:      1,
		}
		require.NoError(t, deletionRemainderDB.Create(ctx, charge1))

		charge2 := accounting.DeletionRemainderCharge{
			ProjectID:      projectID,
			BucketName:     bucketName,
			CreatedAt:      now.Add(-30 * time.Hour),
			DeletedAt:      now.Add(-10 * time.Hour),
			ObjectSize:     mbUint64(50),
			RemainderHours: 480.0,
			ProductID:      1,
		}
		require.NoError(t, deletionRemainderDB.Create(ctx, charge2))

		otherProjectID := testrand.UUID()
		charge3 := accounting.DeletionRemainderCharge{
			ProjectID:      otherProjectID,
			BucketName:     "other-bucket",
			CreatedAt:      now.Add(-30 * time.Hour),
			DeletedAt:      now.Add(-5 * time.Hour),
			ObjectSize:     mbUint64(200),
			RemainderHours: 600.0,
			ProductID:      2,
		}
		require.NoError(t, deletionRemainderDB.Create(ctx, charge3))

		charge4 := accounting.DeletionRemainderCharge{
			ProjectID:      projectID,
			BucketName:     bucketName,
			CreatedAt:      now.Add(-60 * time.Hour),
			DeletedAt:      now.Add(-45 * time.Hour),
			ObjectSize:     mbUint64(75),
			RemainderHours: 360.0,
			ProductID:      1,
			Billed:         true,
		}
		require.NoError(t, deletionRemainderDB.Create(ctx, charge4))

		// Test GetUnbilledCharges - should only get unbilled charges for the project in the time range
		from := now.Add(-20 * time.Hour)
		to := now.Add(-1 * time.Hour)

		charges, err := deletionRemainderDB.GetUnbilledCharges(ctx, projectID, from, to)
		require.NoError(t, err)
		require.Len(t, charges, 2, "should get exactly 2 unbilled charges")

		// Verify the charges
		totalByteHours := 0.0
		for _, charge := range charges {
			require.Equal(t, projectID, charge.ProjectID)
			require.False(t, charge.Billed)
			totalByteHours += charge.RemainderHours * float64(charge.ObjectSize)
		}

		// Calculate expected byte-hours
		expected1 := charge1.RemainderHours * float64(charge1.ObjectSize)
		expected2 := charge2.RemainderHours * float64(charge2.ObjectSize)
		expectedTotal := expected1 + expected2
		require.InDelta(t, expectedTotal, totalByteHours, 1.0, "total byte-hours should match")

		// Test GetUnbilledCharges with different time range - should get none
		earlyFrom := now.Add(-90 * time.Hour)
		earlyTo := now.Add(-60 * time.Hour)

		earlyCharges, err := deletionRemainderDB.GetUnbilledCharges(ctx, projectID, earlyFrom, earlyTo)
		require.NoError(t, err)
		require.Empty(t, earlyCharges, "should get no charges outside time range")

		// Test GetUnbilledCharges for other project
		otherCharges, err := deletionRemainderDB.GetUnbilledCharges(ctx, otherProjectID, from, to)
		require.NoError(t, err)
		require.Len(t, otherCharges, 1, "should get 1 charge for other project")
		require.Equal(t, otherProjectID, otherCharges[0].ProjectID)
		require.Equal(t, int32(2), otherCharges[0].ProductID)

		// Test MarkChargesAsBilled
		err = deletionRemainderDB.MarkChargesAsBilled(ctx, projectID, from, to)
		require.NoError(t, err)

		// Verify charges are now marked as billed
		chargesAfterBilling, err := deletionRemainderDB.GetUnbilledCharges(ctx, projectID, from, to)
		require.NoError(t, err)
		require.Len(t, chargesAfterBilling, 0, "should get no unbilled charges after marking as billed")

		// Verify other project's charges are still unbilled
		otherChargesStillUnbilled, err := deletionRemainderDB.GetUnbilledCharges(ctx, otherProjectID, from, to)
		require.NoError(t, err)
		require.Len(t, otherChargesStillUnbilled, 1, "other project's charges should still be unbilled")
	})
}
