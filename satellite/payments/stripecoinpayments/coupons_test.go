// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestCouponRepository(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		couponsRepo := db.StripeCoinPayments().Coupons()
		coupon := payments.Coupon{
			Duration:    2,
			Amount:      10,
			Status:      payments.CouponActive,
			Description: "description",
			UserID:      testrand.UUID(),
		}

		now := time.Now().UTC()

		t.Run("insert", func(t *testing.T) {
			_, err := couponsRepo.Insert(ctx, coupon)
			require.NoError(t, err)

			coupons, err := couponsRepo.List(ctx, payments.CouponActive)
			require.NoError(t, err)
			require.Equal(t, 1, len(coupons))
			coupon = coupons[0]
		})

		t.Run("update", func(t *testing.T) {
			_, err := couponsRepo.Update(ctx, coupon.ID, payments.CouponUsed)
			require.NoError(t, err)

			coupons, err := couponsRepo.List(ctx, payments.CouponUsed)
			require.NoError(t, err)
			require.Equal(t, payments.CouponUsed, coupons[0].Status)
			coupon = coupons[0]
		})

		t.Run("get latest on empty table return stripecoinpayments.ErrNoCouponUsages", func(t *testing.T) {
			_, err := couponsRepo.GetLatest(ctx, coupon.ID)
			require.Error(t, err)
			require.Equal(t, true, stripecoinpayments.ErrNoCouponUsages.Has(err))
		})

		t.Run("total on empty table returns 0", func(t *testing.T) {
			total, err := couponsRepo.TotalUsage(ctx, coupon.ID)
			require.NoError(t, err)
			require.Equal(t, int64(0), total)
		})

		t.Run("add usage", func(t *testing.T) {
			err := couponsRepo.AddUsage(ctx, stripecoinpayments.CouponUsage{
				CouponID: coupon.ID,
				Amount:   1,
				Period:   now,
			})
			require.NoError(t, err)
			date, err := couponsRepo.GetLatest(ctx, coupon.ID)
			require.NoError(t, err)
			// go and postgres has different precision. go - nanoseconds, postgres micro
			require.Equal(t, date.UTC(), now.Truncate(time.Microsecond))
		})

		t.Run("total usage", func(t *testing.T) {
			amount, err := couponsRepo.TotalUsage(ctx, coupon.ID)
			require.NoError(t, err)
			require.Equal(t, amount, int64(1))
		})
	})
}

// TestPopulatePromotionalCoupons is a test for PopulatePromotionalCoupons function
// that creates coupons with predefined values for each of user (from arguments) that have a project
// and that don't have a promotional coupon yet. Also it updates limits of selected projects to 1TB.
// Because the coupon should be added to a project, we select the first project of the user.
// In this test i have next test cases:
// 1. Activated user, 2 projects, without coupon. For this case we should add new coupon to his first project.
// 2. Activated user, 1 project, without coupon.
// 3. Activated user without project. Coupon should not be added.
// 4. User with inactive account. Coupon should not be added.
// 5. Activated user with project and coupon. Coupon should not be added.
// 6. Next step - is populating coupons for all 5 users. Only 2 coupons should be added.
// 7. Creating new user with project.
// 8. Populating coupons again. For 6 users above. Only 1 new coupon should be added.
// Three new coupons total should be added by 2 runs of PopulatePromotionalCoupons method.
func TestPopulatePromotionalCoupons(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		usersRepo := db.Console().Users()
		projectsRepo := db.Console().Projects()
		couponsRepo := db.StripeCoinPayments().Coupons()
		usageRepo := db.ProjectAccounting()

		// creating test users with different status.

		// activated user with 2 project. New coupon should be added to the first project.
		user1, err := usersRepo.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "user1",
			ShortName:    "",
			Email:        "test1@example.com",
			PasswordHash: []byte("123qwe"),
		})
		require.NoError(t, err)

		user1.Status = console.Active

		err = usersRepo.Update(ctx, user1)
		require.NoError(t, err)

		// activated user with proj. New coupon should be added.
		user2, err := usersRepo.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "user2",
			ShortName:    "",
			Email:        "test2@example.com",
			PasswordHash: []byte("123qwe"),
		})
		require.NoError(t, err)

		user2.Status = console.Active

		err = usersRepo.Update(ctx, user2)
		require.NoError(t, err)

		// activated user without proj. New coupon should not be added.
		user3, err := usersRepo.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "user3",
			ShortName:    "",
			Email:        "test3@example.com",
			PasswordHash: []byte("123qwe"),
		})
		require.NoError(t, err)

		user3.Status = console.Active

		err = usersRepo.Update(ctx, user3)
		require.NoError(t, err)

		// inactive user. New coupon should not be added.
		user4, err := usersRepo.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "user4",
			ShortName:    "",
			Email:        "test4@example.com",
			PasswordHash: []byte("123qwe"),
		})
		require.NoError(t, err)

		// activated user with proj and coupon. New coupon should not be added.
		user5, err := usersRepo.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "user5",
			ShortName:    "",
			Email:        "test5@example.com",
			PasswordHash: []byte("123qwe"),
		})
		require.NoError(t, err)

		user5.Status = console.Active

		err = usersRepo.Update(ctx, user5)
		require.NoError(t, err)

		// creating projects for users above.
		proj1, err := projectsRepo.Insert(ctx, &console.Project{
			ID:          testrand.UUID(),
			Name:        "proj 1 of user 1",
			Description: "descr 1",
			OwnerID:     user1.ID,
		})
		require.NoError(t, err)

		// should not be processed as we takes only first project of the user.
		proj2, err := projectsRepo.Insert(ctx, &console.Project{
			ID:          testrand.UUID(),
			Name:        "proj 2 of user 1",
			Description: "descr 2",
			OwnerID:     user1.ID,
		})
		require.NoError(t, err)

		proj3, err := projectsRepo.Insert(ctx, &console.Project{
			ID:          testrand.UUID(),
			Name:        "proj 1 of user 2",
			Description: "descr 3",
			OwnerID:     user2.ID,
		})
		require.NoError(t, err)

		couponID := testrand.UUID()
		_, err = couponsRepo.Insert(ctx, payments.Coupon{
			ID:          couponID,
			UserID:      user5.ID,
			Amount:      5500,
			Duration:    2,
			Description: "qw",
			Type:        payments.CouponTypePromotional,
			Status:      payments.CouponActive,
		})
		require.NoError(t, err)

		// creating new users and projects to test that multiple execution of populate method wont generate extra coupons.
		user6, err := usersRepo.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "user6",
			ShortName:    "",
			Email:        "test6@example.com",
			PasswordHash: []byte("123qwe"),
		})
		require.NoError(t, err)

		user6.Status = console.Active

		err = usersRepo.Update(ctx, user6)
		require.NoError(t, err)

		proj5, err := projectsRepo.Insert(ctx, &console.Project{
			ID:          testrand.UUID(),
			Name:        "proj 1 of user 6",
			Description: "descr 6",
			OwnerID:     user6.ID,
		})
		if err != nil {
			require.NoError(t, err)
		}

		t.Run("first population", func(t *testing.T) {
			var usersIds = []uuid.UUID{
				user1.ID,
				user2.ID,
				user3.ID,
				user4.ID,
				user5.ID,
			}
			err := couponsRepo.PopulatePromotionalCoupons(ctx, usersIds, 2, 5500, memory.TB)
			require.NoError(t, err)

			user1Coupons, err := couponsRepo.ListByUserID(ctx, user1.ID)
			require.NoError(t, err)
			require.Equal(t, 1, len(user1Coupons))

			proj1Usage, err := usageRepo.GetProjectStorageLimit(ctx, proj1.ID)
			require.NoError(t, err)
			require.Equal(t, memory.TB.Int64(), *proj1Usage)

			proj2Usage, err := usageRepo.GetProjectStorageLimit(ctx, proj2.ID)
			require.NoError(t, err)
			require.Nil(t, proj2Usage)

			user2Coupons, err := couponsRepo.ListByUserID(ctx, user2.ID)
			require.NoError(t, err)
			require.Equal(t, 1, len(user2Coupons))

			proj3Usage, err := usageRepo.GetProjectStorageLimit(ctx, proj3.ID)
			require.NoError(t, err)
			require.Equal(t, memory.TB.Int64(), *proj3Usage)

			user3Coupons, err := couponsRepo.ListByUserID(ctx, user3.ID)
			require.NoError(t, err)
			require.Equal(t, 0, len(user3Coupons))

			user4Coupons, err := couponsRepo.ListByUserID(ctx, user4.ID)
			require.NoError(t, err)
			require.Equal(t, 0, len(user4Coupons))

			user5Coupons, err := couponsRepo.ListByUserID(ctx, user5.ID)
			require.NoError(t, err)
			require.Equal(t, 1, len(user5Coupons))
			require.Equal(t, "qw", user5Coupons[0].Description)
		})

		t.Run("second population", func(t *testing.T) {
			var usersIds = []uuid.UUID{
				user1.ID,
				user2.ID,
				user3.ID,
				user4.ID,
				user5.ID,
				user6.ID,
			}
			err := couponsRepo.PopulatePromotionalCoupons(ctx, usersIds, 2, 5500, memory.TB)
			require.NoError(t, err)

			user1Coupons, err := couponsRepo.ListByUserID(ctx, user1.ID)
			require.NoError(t, err)
			require.Equal(t, 1, len(user1Coupons))

			proj1Usage, err := usageRepo.GetProjectStorageLimit(ctx, proj1.ID)
			require.NoError(t, err)
			require.Equal(t, memory.TB.Int64(), *proj1Usage)

			proj2Usage, err := usageRepo.GetProjectStorageLimit(ctx, proj2.ID)
			require.NoError(t, err)
			require.Nil(t, proj2Usage)

			user2Coupons, err := couponsRepo.ListByUserID(ctx, user2.ID)
			require.NoError(t, err)
			require.Equal(t, 1, len(user2Coupons))

			proj3Usage, err := usageRepo.GetProjectStorageLimit(ctx, proj3.ID)
			require.NoError(t, err)
			require.Equal(t, memory.TB.Int64(), *proj3Usage)

			user3Coupons, err := couponsRepo.ListByUserID(ctx, user3.ID)
			require.NoError(t, err)
			require.Equal(t, 0, len(user3Coupons))

			user4Coupons, err := couponsRepo.ListByUserID(ctx, user4.ID)
			require.NoError(t, err)
			require.Equal(t, 0, len(user4Coupons))

			user5Coupons, err := couponsRepo.ListByUserID(ctx, user5.ID)
			require.NoError(t, err)
			require.Equal(t, 1, len(user5Coupons))
			require.Equal(t, "qw", user5Coupons[0].Description)

			user6Coupons, err := couponsRepo.ListByUserID(ctx, user6.ID)
			require.NoError(t, err)
			require.Equal(t, 1, len(user6Coupons))

			proj5Usage, err := usageRepo.GetProjectStorageLimit(ctx, proj5.ID)
			require.NoError(t, err)
			require.Equal(t, memory.TB.Int64(), *proj5Usage)
		})
	})
}
