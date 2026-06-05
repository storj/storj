// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
)

func TestInactivityFreezeChore(t *testing.T) {
	gracePeriod := 2 * time.Hour
	now := time.Now().UTC().Truncate(time.Second)
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	prevMonthStart := currentMonthStart.AddDate(0, -1, 0)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.AccountFreeze.Enabled = true
				config.AccountFreeze.EmailsEnabled = true
				config.AccountFreeze.InactivitySuspendEnabled = true
				config.AccountFreeze.InactivityConsecutiveZeroCycles = 2
				config.Console.AccountFreeze.InactivityGracePeriod = gracePeriod
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		stripeClient := sat.API.Payments.StripeClient
		customerDB := sat.Core.DB.StripeCoinPayments().Customers()
		usersDB := sat.DB.Console().Users()

		service := console.NewAccountFreezeService(sat.DB.Console(), newFreezeTrackerMock(t), sat.Config.Console.AccountFreeze)
		service.TestSetInactivityGracePeriod(gracePeriod)

		chore := sat.Core.AccountFreeze.InactivityFreezeChore
		chore.Loop.Pause()
		chore.TestSetFreezeService(service)
		chore.TestSetNow(func() time.Time { return now })

		newPaidUser := func(t *testing.T, email string) *console.User {
			t.Helper()
			u, err := sat.AddUser(ctx, console.CreateUser{FullName: "Test User", Email: email}, 1)
			require.NoError(t, err)
			paidKind := console.PaidUser
			// Set UpgradeTime far enough in the past to clear the N-cycle recency guard.
			upgradeTime := now.AddDate(0, -(sat.Config.AccountFreeze.InactivityConsecutiveZeroCycles + 1), 0)
			upgradeTimePtr := &upgradeTime
			require.NoError(t, usersDB.Update(ctx, u.ID, console.UpdateUserRequest{Kind: &paidKind, UpgradeTime: &upgradeTimePtr}))
			return u
		}

		addInvoice := func(t *testing.T, userID uuid.UUID, amount int64, periodStart *time.Time) {
			t.Helper()
			cusID, err := customerDB.GetCustomerID(ctx, userID)
			require.NoError(t, err)

			meta := map[string]string{}
			if periodStart != nil {
				meta["_period_start"] = fmt.Sprintf("%d", periodStart.Unix())
			}
			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:   stripe.Params{Context: ctx},
				Customer: &cusID,
				Metadata: meta,
			})
			require.NoError(t, err)

			if amount > 0 {
				curr := string(stripe.CurrencyUSD)
				_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
					Params:   stripe.Params{Context: ctx},
					Amount:   &amount,
					Currency: &curr,
					Customer: &cusID,
					Invoice:  &inv.ID,
				})
				require.NoError(t, err)
			}
		}

		t.Run("warns PaidUser with no invoices over N cycles", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			user := newPaidUser(t, "zero-invoices@mail.test")

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.InactivityWarning, "user with no invoices should receive InactivityWarning")
			require.Nil(t, freezes.InactivityFreeze)
		})

		t.Run("does not warn user with non-zero invoice in checked window", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			user := newPaidUser(t, "nonzero-invoice@mail.test")

			// Put a non-zero invoice in the middle of the previous complete month.
			prevMonthMid := prevMonthStart.Add(15 * 24 * time.Hour)
			addInvoice(t, user.ID, 100, &prevMonthMid)

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.InactivityWarning, "user with non-zero invoice should not be warned")
		})

		t.Run("does not warn user with billable accounting usage (no Stripe customer)", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			// Insert a paid user without calling SetupAccount, so no Stripe customer is created.
			userID := testrand.UUID()
			_, err := usersDB.Insert(ctx, &console.User{
				ID:           userID,
				FullName:     "Accounting User",
				Email:        "accounting-usage@mail.test",
				PasswordHash: []byte("password"),
			})
			require.NoError(t, err)

			activeStatus := console.Active
			paidKind := console.PaidUser
			upgradeTime := now.AddDate(0, -(sat.Config.AccountFreeze.InactivityConsecutiveZeroCycles + 1), 0)
			upgradeTimePtr := &upgradeTime
			require.NoError(t, usersDB.Update(ctx, userID, console.UpdateUserRequest{
				Status:      &activeStatus,
				Kind:        &paidKind,
				UpgradeTime: &upgradeTimePtr,
			}))

			project, err := sat.AddProject(ctx, userID, "accounting-project")
			require.NoError(t, err)

			prevMonthMid := prevMonthStart.Add(15 * 24 * time.Hour)
			for _, at := range []time.Time{prevMonthMid, prevMonthMid.Add(time.Hour)} {
				require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
					BucketName:    "test-bucket",
					ProjectID:     project.ID,
					IntervalStart: at,
					TotalBytes:    1000 * 1024 * 1024 * 1024, // 1 TB
				}))
			}

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, userID)
			require.NoError(t, err)
			require.Nil(t, freezes.InactivityWarning, "user with billable accounting usage should not be warned")
		})

		t.Run("warns user with sub-billable accounting usage (no Stripe customer)", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			// Insert a paid user without a Stripe customer.
			userID := testrand.UUID()
			_, err := usersDB.Insert(ctx, &console.User{
				ID:           userID,
				FullName:     "Sub-billable User",
				Email:        "sub-billable@mail.test",
				PasswordHash: []byte("password"),
			})
			require.NoError(t, err)

			activeStatus := console.Active
			paidKind := console.PaidUser
			upgradeTime := now.AddDate(0, -(sat.Config.AccountFreeze.InactivityConsecutiveZeroCycles + 1), 0)
			upgradeTimePtr := &upgradeTime
			require.NoError(t, usersDB.Update(ctx, userID, console.UpdateUserRequest{
				Status:      &activeStatus,
				Kind:        &paidKind,
				UpgradeTime: &upgradeTimePtr,
			}))

			project, err := sat.AddProject(ctx, userID, "sub-billable-project")
			require.NoError(t, err)

			prevMonthMid := prevMonthStart.Add(15 * 24 * time.Hour)
			for _, at := range []time.Time{prevMonthMid, prevMonthMid.Add(time.Hour)} {
				require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
					BucketName:    "test-bucket",
					ProjectID:     project.ID,
					IntervalStart: at,
					TotalBytes:    100 * 1024 * 1024, // 100 MB
				}))
			}

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, userID)
			require.NoError(t, err)
			require.NotNil(t, freezes.InactivityWarning, "user with sub-billable accounting usage should be warned")
		})

		t.Run("zero-amount invoice does not prevent warning", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			user := newPaidUser(t, "zero-amount-invoice@mail.test")

			// Invoice with zero amount — treated as zero revenue.
			prevMonthMid := prevMonthStart.Add(15 * 24 * time.Hour)
			addInvoice(t, user.ID, 0, &prevMonthMid)

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.InactivityWarning, "user with zero-amount invoice should still be warned")
		})

		t.Run("warn escalates to freeze after grace period", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			user := newPaidUser(t, "escalate@mail.test")

			// Phase 1: warn.
			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.InactivityWarning)
			warnCreatedAt := freezes.InactivityWarning.CreatedAt

			// Phase 2: advance past the grace period with no new usage → freeze.
			chore.TestSetNow(func() time.Time { return warnCreatedAt.Add(gracePeriod + time.Hour) })
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.Loop.TriggerWait()

			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.InactivityWarning, "InactivityWarning should be removed on freeze")
			require.NotNil(t, freezes.InactivityFreeze, "user should have InactivityFreeze event")

			frozenUser, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.EqualValues(t, 0, frozenUser.ProjectStorageLimit, "storage limit should be zeroed on freeze")
			require.EqualValues(t, 0, frozenUser.ProjectBandwidthLimit, "bandwidth limit should be zeroed on freeze")
			require.EqualValues(t, 0, frozenUser.ProjectSegmentLimit, "segment limit should be zeroed on freeze")
		})

		t.Run("grace period is respected before freeze", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			user := newPaidUser(t, "grace-respected@mail.test")

			// Phase 1: warn.
			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.InactivityWarning)
			warnCreatedAt := freezes.InactivityWarning.CreatedAt

			// Advance to within grace — user should not be frozen yet.
			chore.TestSetNow(func() time.Time { return warnCreatedAt.Add(gracePeriod / 2) })
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.Loop.TriggerWait()

			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.InactivityWarning, "warning should persist within grace period")
			require.Nil(t, freezes.InactivityFreeze, "user should not be frozen within grace period")
		})

		t.Run("cancels warning when billable usage detected during grace period", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			user := newPaidUser(t, "cancel-warning@mail.test")
			project, err := sat.AddProject(ctx, user.ID, "cancel-warning-project")
			require.NoError(t, err)

			// Phase 1: warn.
			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.InactivityWarning)
			warnCreatedAt := freezes.InactivityWarning.CreatedAt

			tallyStart := warnCreatedAt.Add(time.Second)
			for _, at := range []time.Time{tallyStart, tallyStart.Add(30 * time.Minute)} {
				require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
					BucketName:    "test-bucket",
					ProjectID:     project.ID,
					IntervalStart: at,
					TotalBytes:    1000 * 1024 * 1024 * 1024, // 1 TB
				}))
			}

			// Advance to within grace period.
			chore.TestSetNow(func() time.Time { return warnCreatedAt.Add(gracePeriod / 2) })
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.Loop.TriggerWait()

			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.InactivityWarning, "warning should be cancelled when billable usage is detected")
			require.Nil(t, freezes.InactivityFreeze)
		})

		t.Run("does not cancel warning when resumed usage is not billable", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			user := newPaidUser(t, "sub-billable-resume@mail.test")
			project, err := sat.AddProject(ctx, user.ID, "sub-billable-resume-project")
			require.NoError(t, err)

			// Phase 1: warn.
			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.InactivityWarning)
			warnCreatedAt := freezes.InactivityWarning.CreatedAt

			tallyStart := warnCreatedAt.Add(time.Second)
			for _, at := range []time.Time{tallyStart, tallyStart.Add(30 * time.Minute)} {
				require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
					BucketName:    "test-bucket",
					ProjectID:     project.ID,
					IntervalStart: at,
					TotalBytes:    100 * 1024 * 1024, // 100 MB
				}))
			}

			// Advance to within grace period.
			chore.TestSetNow(func() time.Time { return warnCreatedAt.Add(gracePeriod / 2) })
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.Loop.TriggerWait()

			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.InactivityWarning, "warning should persist when resumed usage is sub-billable")
			require.Nil(t, freezes.InactivityFreeze)
		})

		t.Run("does not warn inactivity-exempt user", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			user := newPaidUser(t, "inactivity-exempt@mail.test")

			exempt := true
			require.NoError(t, usersDB.UpsertSettings(ctx, user.ID, console.UpsertUserSettingsRequest{
				InactivityExempt: &exempt,
			}))

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.InactivityWarning, "inactivity-exempt user should not be warned")
		})

		t.Run("does not warn free (billing-exempt) user", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			freeUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Free User", Email: "freeuser-inactivity@mail.test",
			}, 1)
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, freeUser.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.InactivityWarning, "free user should not be warned")
		})

		t.Run("does not warn recently upgraded user", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			user := newPaidUser(t, "recent-upgrade@mail.test")

			// Override UpgradeTime to within the check window
			// (less than InactivityConsecutiveZeroCycles months ago).
			recentUpgrade := now.AddDate(0, -(sat.Config.AccountFreeze.InactivityConsecutiveZeroCycles - 1), 0)
			recentUpgradePtr := &recentUpgrade
			require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{UpgradeTime: &recentUpgradePtr}))

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.InactivityWarning, "recently upgraded user should not be warned")
		})

		t.Run("does not warn user with existing BillingFreeze", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			user := newPaidUser(t, "billing-frozen-skip@mail.test")
			require.NoError(t, service.BillingFreezeUser(ctx, user.ID))

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.InactivityWarning, "billing-frozen user should not receive inactivity warning")
			require.NotNil(t, freezes.BillingFreeze, "billing freeze should be unchanged")
		})
	})
}
