// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v75"
	"go.uber.org/zap"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/accountfreeze"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/storjscan/blockchaintest"
	stripe1 "storj.io/storj/satellite/payments/stripe"
)

func TestAutoFreezeChore(t *testing.T) {
	warnEmailsIntervals := accountfreeze.EmailIntervals{
		240 * time.Hour,
		96 * time.Hour,
	}
	freezeEmailsIntervals := accountfreeze.EmailIntervals{
		720 * time.Hour,
		480 * time.Hour,
		216 * time.Hour,
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.AccountFreeze.Enabled = true
				config.Console.Captcha.FlagBotsEnabled = true
				config.AccountFreeze.EmailsEnabled = true
				config.AccountFreeze.BillingWarningEmailIntervals = warnEmailsIntervals
				config.AccountFreeze.BillingFreezeEmailIntervals = freezeEmailsIntervals
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		stripeClient := sat.API.Payments.StripeClient
		invoicesDB := sat.Core.Payments.Accounts.Invoices()
		customerDB := sat.Core.DB.StripeCoinPayments().Customers()
		usersDB := sat.DB.Console().Users()
		accFreezeDB := sat.DB.Console().AccountFreezeEvents()
		service := console.NewAccountFreezeService(sat.DB.Console(), newFreezeTrackerMock(t), sat.Config.Console.AccountFreeze)
		chore := sat.Core.Payments.AccountFreeze

		chore.Loop.Pause()
		chore.TestSetFreezeService(service)

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 1)
		require.NoError(t, err)

		cus1, err := customerDB.GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		amount := int64(100)
		curr := string(stripe.CurrencyUSD)

		t.Run("No billing event for legal frozen user", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))

			violatingUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Violating User",
				Email:    "legalhold@mail.test",
			}, 1)
			require.NoError(t, err)

			cus2, err := customerDB.GetCustomerID(ctx, violatingUser.ID)
			require.NoError(t, err)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:   stripe.Params{Context: ctx},
				Customer: &cus2,
			})
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus2,
				Invoice:  &inv.ID,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPayFailure
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.Error(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 1, len(failed))

			require.NoError(t, service.LegalFreezeUser(ctx, violatingUser.ID))

			chore.Loop.TriggerWait()

			// user should not be billing warned or frozen.
			freezes, err := service.GetAll(ctx, violatingUser.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingWarning)
			require.Nil(t, freezes.BillingFreeze)
			require.NotNil(t, freezes.LegalFreeze)

			// forward date to after the grace period
			chore.TestSetNow(func() time.Time {
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingWarnGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// user should still not be billing warned or frozen.
			freezes, err = service.GetAll(ctx, violatingUser.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.BillingWarning)
			require.NotNil(t, freezes.LegalFreeze)

			paymentMethod = stripe1.MockInvoicesPaySuccess
			_, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusPaid, inv.Status)

			chore.Loop.TriggerWait()

			// paying for the invoice does not remove the legal freeze
			freezes, err = service.GetAll(ctx, violatingUser.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.BillingWarning)
			require.NotNil(t, freezes.LegalFreeze)
		})

		t.Run("No billing event for violation frozen user", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))

			violatingUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Violating User",
				Email:    "violating@mail.test",
			}, 1)
			require.NoError(t, err)

			cus2, err := customerDB.GetCustomerID(ctx, violatingUser.ID)
			require.NoError(t, err)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:   stripe.Params{Context: ctx},
				Customer: &cus2,
			})
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus2,
				Invoice:  &inv.ID,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPayFailure
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.Error(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 1, len(failed))

			require.NoError(t, service.ViolationFreezeUser(ctx, violatingUser.ID))

			chore.Loop.TriggerWait()

			// user should not be billing warned or frozen.
			freezes, err := service.GetAll(ctx, violatingUser.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingWarning)
			require.Nil(t, freezes.BillingFreeze)
			require.NotNil(t, freezes.ViolationFreeze)

			// forward date to after the grace period
			chore.TestSetNow(func() time.Time {
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingWarnGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// user should still not be billing warned or frozen.
			freezes, err = service.GetAll(ctx, violatingUser.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.BillingWarning)
			require.NotNil(t, freezes.ViolationFreeze)

			paymentMethod = stripe1.MockInvoicesPaySuccess
			_, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusPaid, inv.Status)

			chore.Loop.TriggerWait()

			// paying for the invoice does not remove the violation freeze
			freezes, err = service.GetAll(ctx, violatingUser.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.BillingWarning)
			require.NotNil(t, freezes.ViolationFreeze)
		})

		t.Run("No billing freeze event for paid invoice", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			_, err := stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus1,
			})
			require.NoError(t, err)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:   stripe.Params{Context: ctx},
				Customer: &cus1,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPaySuccess
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusPaid, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 0, len(failed))

			chore.Loop.TriggerWait()

			// user should not be warned or frozen.
			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.BillingWarning)
			require.Nil(t, freezes.ViolationFreeze)

			// forward date to after the grace period
			chore.TestSetNow(func() time.Time {
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingWarnGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// user should still not be warned or frozen.
			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.BillingWarning)
			require.Nil(t, freezes.ViolationFreeze)
		})

		t.Run("BillingFreeze event for failed invoice (failed later payment attempt)", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			// reset chore clock
			chore.TestSetNow(time.Now)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:   stripe.Params{Context: ctx},
				Customer: &cus1,
			})
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus1,
				Invoice:  &inv.ID,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPayFailure
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.Error(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 1, len(failed))
			require.Equal(t, inv.ID, failed[0].ID)

			chore.Loop.TriggerWait()

			// user should be warned the first time
			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.BillingWarning)
			// user should be notified once when this event happens for the first time.
			require.Equal(t, 1, freezes.BillingWarning.NotificationsCount)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.ViolationFreeze)

			chore.TestSetNow(func() time.Time {
				// current date is now after billing warn grace period
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingWarnGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// user should be frozen this time around
			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.BillingFreeze)
			// user should be notified once when this event happens for the first time.
			require.Equal(t, 1, freezes.BillingFreeze.NotificationsCount)

			chore.TestSetNow(func() time.Time {
				// current date is now after billing freeze grace period
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingFreezeGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// user should be marked for deletion after the grace period
			// after being frozen
			userPD, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, console.PendingDeletion, userPD.Status)

			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.BillingFreeze)
			// the billing freeze event should have escalation disabled.
			require.Nil(t, freezes.BillingFreeze.DaysTillEscalation)

			// Pay invoice so user qualifies to be removed from billing freeze.
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: stripe.String(stripe1.MockInvoicesPaySuccess),
			})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusPaid, inv.Status)

			// set user status to deleted
			status := console.Deleted
			err = usersDB.Update(ctx, user.ID, console.UpdateUserRequest{
				Status: &status,
			})
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// deleted user should be skipped, hence would not exist the
			// billing freeze status.
			isFrozen, err := service.IsUserBillingFrozen(ctx, user.ID)
			require.NoError(t, err)
			require.True(t, isFrozen)

			// unfreeze user so they're not frozen in the next test.
			err = service.BillingUnfreezeUser(ctx, user.ID)
			require.NoError(t, err)

			// set user status back to active
			status = console.Active
			err = usersDB.Update(ctx, user.ID, console.UpdateUserRequest{
				Status: &status,
			})
			require.NoError(t, err)
		})

		t.Run("No freeze event for failed invoice (successful later payment attempt)", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			// reset chore clock
			chore.TestSetNow(time.Now)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:               stripe.Params{Context: ctx},
				Customer:             &cus1,
				DefaultPaymentMethod: stripe.String(stripe1.MockInvoicesPaySuccess),
			})
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus1,
				Invoice:  &inv.ID,
			})
			require.NoError(t, err)

			inv, err = stripeClient.Invoices().FinalizeInvoice(inv.ID, nil)
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 1, len(failed))
			require.Equal(t, inv.ID, failed[0].ID)

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.BillingWarning)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.ViolationFreeze)

			chore.TestSetNow(func() time.Time {
				// current date is now after billing warn grace period
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingWarnGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// Payment should have succeeded in the chore.
			failed, err = invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 0, len(failed))

			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.BillingWarning)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.ViolationFreeze)
		})

		t.Run("User unfrozen/unwarned for no failed invoices", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			// reset chore clock
			chore.TestSetNow(time.Now)

			user2, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test User",
				Email:    "user2@mail.test",
			}, 1)
			require.NoError(t, err)

			cus2, err := customerDB.GetCustomerID(ctx, user2.ID)
			require.NoError(t, err)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:   stripe.Params{Context: ctx},
				Customer: &cus2,
			})
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus2,
				Invoice:  &inv.ID,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPayFailure
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.Error(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 1, len(failed))

			err = service.BillingFreezeUser(ctx, user.ID)
			require.NoError(t, err)
			err = service.BillingFreezeUser(ctx, user2.ID)
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// user(1) should be unfrozen because they have no failed invoices
			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.BillingFreeze)

			// user2 should still be frozen because they have failed invoices
			freezes, err = service.GetAll(ctx, user2.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.BillingFreeze)

			// warn user though they have no failed invoices
			err = service.BillingWarnUser(ctx, user.ID)
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// warned status should be reset
			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.BillingWarning)

			// Pay invoice so it doesn't show up in the next test.
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: stripe.String(stripe1.MockInvoicesPaySuccess),
			})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusPaid, inv.Status)

			// unfreeze user so they're not frozen in the next test.
			err = service.BillingUnfreezeUser(ctx, user2.ID)
			require.NoError(t, err)
		})

		t.Run("Bot user is frozen with delay", func(t *testing.T) {
			// reset chore clock
			chore.TestSetNow(time.Now)

			botUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test Bot User",
				Email:    "botuser@mail.test",
			}, 2)
			require.NoError(t, err)

			_, err = sat.AddProject(ctx, botUser.ID, "test")
			require.NoError(t, err)

			_, err = sat.AddProject(ctx, botUser.ID, "test1")
			require.NoError(t, err)

			two := 2
			_, err = accFreezeDB.Upsert(ctx, &console.AccountFreezeEvent{
				UserID:             botUser.ID,
				Type:               console.DelayedBotFreeze,
				DaysTillEscalation: &two,
			})
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// User is not bot frozen yet.
			botUser, err = usersDB.Get(ctx, botUser.ID)
			require.NoError(t, err)
			require.Equal(t, console.Active, botUser.Status)

			// Delayed event still exists.
			event, err := accFreezeDB.Get(ctx, botUser.ID, console.DelayedBotFreeze)
			require.NoError(t, err)
			require.NotNil(t, event)
			require.Equal(t, console.DelayedBotFreeze, event.Type)

			// Current date is now 3 days later.
			chore.TestSetNow(func() time.Time {
				return time.Now().Add(24 * 3 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// User is bot frozen and has zero limits.
			botUser, err = usersDB.Get(ctx, botUser.ID)
			require.NoError(t, err)
			require.Equal(t, console.PendingBotVerification, botUser.Status)
			require.Zero(t, botUser.ProjectBandwidthLimit)
			require.Zero(t, botUser.ProjectStorageLimit)
			require.Zero(t, botUser.ProjectSegmentLimit)

			// Users projects have zero limits.
			projects, err := sat.DB.Console().Projects().GetOwn(ctx, botUser.ID)
			require.NoError(t, err)
			require.Len(t, projects, 2)

			for _, p := range projects {
				require.Zero(t, *p.StorageLimit)
				require.Zero(t, *p.BandwidthLimit)
				require.Zero(t, *p.SegmentLimit)
				require.Zero(t, *p.RateLimit)
			}

			// BotFreeze event was inserted.
			event, err = accFreezeDB.Get(ctx, botUser.ID, console.BotFreeze)
			require.NoError(t, err)
			require.NotNil(t, event)
			require.NotNil(t, event.Limits)

			// Delayed event doesn't exist anymore.
			event, err = accFreezeDB.Get(ctx, botUser.ID, console.DelayedBotFreeze)
			require.Error(t, err)
			require.Nil(t, event)
		})

		t.Run("Free trial expiration freeze", func(t *testing.T) {
			freeUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Free User",
				Email:    "free@mail.test",
			}, 1)
			require.NoError(t, err)
			require.Nil(t, freeUser.TrialExpiration)

			chore.Loop.TriggerWait()

			// user with nil trial expiration should not be frozen.
			frozen, err := service.IsUserFrozen(ctx, freeUser.ID, console.TrialExpirationFreeze)
			require.NoError(t, err)
			require.False(t, frozen)

			now := time.Now()
			newTime := now.Add(120 * time.Hour)
			newTimePtr := &newTime
			err = usersDB.Update(ctx, freeUser.ID, console.UpdateUserRequest{
				TrialExpiration: &newTimePtr,
			})
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// user with future trial expiration should not be frozen.
			frozen, err = service.IsUserFrozen(ctx, freeUser.ID, console.TrialExpirationFreeze)
			require.NoError(t, err)
			require.False(t, frozen)

			// forward date to after newTime
			chore.TestSetNow(func() time.Time {
				return newTime.Add(200 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// user with past trial expiration should be frozen.
			frozen, err = service.IsUserFrozen(ctx, freeUser.ID, console.TrialExpirationFreeze)
			require.NoError(t, err)
			require.True(t, frozen)

			// reset chore time
			chore.TestSetNow(time.Now)

			err = service.TrialExpirationUnfreezeUser(ctx, freeUser.ID)
			require.NoError(t, err)

			// set past expiry and paid tier
			newTime = now.Add(-120 * time.Hour)
			newTimePtr = &newTime
			paidTier := true
			err = usersDB.Update(ctx, freeUser.ID, console.UpdateUserRequest{
				TrialExpiration: &newTimePtr,
				PaidTier:        &paidTier,
			})
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// user with past trial expiration but in paid tier should not be frozen.
			frozen, err = service.IsUserFrozen(ctx, freeUser.ID, console.TrialExpirationFreeze)
			require.NoError(t, err)
			require.False(t, frozen)
		})

		t.Run("Email notifications for events", func(t *testing.T) {
			// reset chore clock
			chore.TestSetNow(time.Now)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:   stripe.Params{Context: ctx},
				Customer: &cus1,
			})
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus1,
				Invoice:  &inv.ID,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPayFailure
			_, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.Error(t, err)

			chore.Loop.TriggerWait()

			warning, err := service.Get(ctx, user.ID, console.BillingWarning)
			require.NoError(t, err)
			require.NotNil(t, warning)
			// user should be notified once when this event happens for the first time.
			require.Equal(t, 1, warning.NotificationsCount)

			chore.TestSetNow(func() time.Time {
				return time.Now().Add(warnEmailsIntervals[0]).Add(1 * time.Minute)
			})
			chore.Loop.TriggerWait()

			warning, err = service.Get(ctx, user.ID, console.BillingWarning)
			require.NoError(t, err)
			// second email should be sent.
			require.Equal(t, 2, warning.NotificationsCount)

			chore.TestSetNow(func() time.Time {
				return time.Now().Add(warnEmailsIntervals[1]).Add(1 * time.Minute)
			})
			chore.Loop.TriggerWait()

			warning, err = service.Get(ctx, user.ID, console.BillingWarning)
			require.NoError(t, err)
			// 3rd email should be sent.
			require.Equal(t, 3, warning.NotificationsCount)

			chore.TestSetNow(func() time.Time {
				// current date is now after billing freeze grace period
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingFreezeGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// no warning should be sent after the grace period
			_, err = service.Get(ctx, user.ID, console.BillingWarning)
			require.Error(t, err)

			freeze, err := service.Get(ctx, user.ID, console.BillingFreeze)
			require.NoError(t, err)
			require.NotNil(t, freeze)
			// user should be notified once when this event happens for the first time.
			require.Equal(t, 1, freeze.NotificationsCount)

			chore.TestSetNow(func() time.Time {
				return time.Now().Add(freezeEmailsIntervals[0]).Add(1 * time.Minute)
			})
			chore.Loop.TriggerWait()

			freeze, err = service.Get(ctx, user.ID, console.BillingFreeze)
			require.NoError(t, err)
			// second email should be sent
			require.Equal(t, 2, freeze.NotificationsCount)

			chore.TestSetNow(func() time.Time {
				return time.Now().Add(freezeEmailsIntervals[1]).Add(1 * time.Minute)
			})
			chore.Loop.TriggerWait()

			freeze, err = service.Get(ctx, user.ID, console.BillingFreeze)
			require.NoError(t, err)
			// third email should be sent
			require.Equal(t, 3, freeze.NotificationsCount)

			chore.TestSetNow(func() time.Time {
				return time.Now().Add(freezeEmailsIntervals[2]).Add(1 * time.Minute)
			})
			chore.Loop.TriggerWait()

			freeze, err = service.Get(ctx, user.ID, console.BillingFreeze)
			require.NoError(t, err)
			// fourth email should be sent
			require.Equal(t, 4, freeze.NotificationsCount)

			chore.TestSetNow(func() time.Time {
				// current date is now after billing freeze grace period
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingFreezeGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			freeze, err = service.Get(ctx, user.ID, console.BillingFreeze)
			require.NoError(t, err)
			// no email after the fourth email
			require.Equal(t, 4, freeze.NotificationsCount)
		})
	})
}

func TestAutoFreezeChore_StorjscanExclusion(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.AccountFreeze.Enabled = true
				config.AccountFreeze.ExcludeStorjscan = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		stripeClient := sat.API.Payments.StripeClient
		invoicesDB := sat.Core.Payments.Accounts.Invoices()
		customerDB := sat.Core.DB.StripeCoinPayments().Customers()
		service := console.NewAccountFreezeService(sat.DB.Console(), newFreezeTrackerMock(t), sat.Config.Console.AccountFreeze)
		chore := sat.Core.Payments.AccountFreeze

		chore.Loop.Pause()
		chore.TestSetFreezeService(service)

		amount := int64(100)
		curr := string(stripe.CurrencyUSD)

		// AnalyticsMock tests that events are sent once.
		service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
		// reset chore clock
		chore.TestSetNow(time.Now)

		storjscanUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "storjscanuser@mail.test",
		}, 1)
		require.NoError(t, err)

		// create a wallet and transaction for the new user in storjscan
		address, err := blockchain.BytesToAddress(testrand.Bytes(20))
		require.NoError(t, err)
		require.NoError(t, sat.DB.Wallets().Add(ctx, storjscanUser.ID, address))
		cachedPayments := []storjscan.CachedPayment{
			{
				From:        blockchaintest.NewAddress(),
				To:          address,
				TokenValue:  currency.AmountFromBaseUnits(1000, currency.StorjToken),
				USDValue:    currency.AmountFromBaseUnits(testrand.Int63n(1000), currency.USDollarsMicro),
				BlockHash:   blockchaintest.NewHash(),
				Transaction: blockchaintest.NewHash(),
				Status:      payments.PaymentStatusConfirmed,
				Timestamp:   time.Now(),
			},
		}
		require.NoError(t, sat.DB.StorjscanPayments().InsertBatch(ctx, cachedPayments))

		storjscanCus, err := customerDB.GetCustomerID(ctx, storjscanUser.ID)
		require.NoError(t, err)

		inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &storjscanCus,
		})
		require.NoError(t, err)

		_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Params:   stripe.Params{Context: ctx},
			Amount:   &amount,
			Currency: &curr,
			Customer: &storjscanCus,
			Invoice:  &inv.ID,
		})
		require.NoError(t, err)

		paymentMethod := stripe1.MockInvoicesPayFailure
		inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
			Params:        stripe.Params{Context: ctx},
			PaymentMethod: &paymentMethod,
		})
		require.Error(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

		failed, err := invoicesDB.ListFailed(ctx, nil)
		require.NoError(t, err)
		require.Equal(t, 1, len(failed))
		invFound := false
		for _, failedInv := range failed {
			if failedInv.ID == inv.ID {
				invFound = true
				break
			}
		}
		require.True(t, invFound)

		chore.Loop.TriggerWait()

		// user should not be warned or frozen due to storjscan payments
		freezes, err := service.GetAll(ctx, storjscanUser.ID)
		require.NoError(t, err)
		require.Nil(t, freezes.BillingWarning)
		require.Nil(t, freezes.BillingFreeze)
		require.Nil(t, freezes.ViolationFreeze)
	})
}

type freezeTrackerMock struct {
	t            *testing.T
	freezeCounts map[string]int
	warnCounts   map[string]int
}

func newFreezeTrackerMock(t *testing.T) *freezeTrackerMock {
	return &freezeTrackerMock{
		t:            t,
		freezeCounts: map[string]int{},
		warnCounts:   map[string]int{},
	}
}

// The following functions are implemented from analytics.FreezeTracker.
// They mock/test to make sure freeze events are sent just once.

func (mock *freezeTrackerMock) TrackAccountFrozen(_ uuid.UUID, email string) {
	mock.freezeCounts[email]++
	// make sure this tracker has not been called already for this email.
	require.Equal(mock.t, 1, mock.freezeCounts[email])
}

func (mock *freezeTrackerMock) TrackAccountUnfrozen(_ uuid.UUID, email string) {
	mock.freezeCounts[email]--
	// make sure this tracker has not been called already for this email.
	require.Equal(mock.t, 0, mock.freezeCounts[email])
}

func (mock *freezeTrackerMock) TrackAccountUnwarned(_ uuid.UUID, email string) {
	mock.warnCounts[email]--
	// make sure this tracker has not been called already for this email.
	require.Equal(mock.t, 0, mock.warnCounts[email])
}

func (mock *freezeTrackerMock) TrackAccountFreezeWarning(_ uuid.UUID, email string) {
	mock.warnCounts[email]++
	// make sure this tracker has not been called already for this email.
	require.Equal(mock.t, 1, mock.warnCounts[email])
}

func (mock *freezeTrackerMock) TrackLargeUnpaidInvoice(_ string, _ uuid.UUID, _ string) {}

func (mock *freezeTrackerMock) TrackStorjscanUnpaidInvoice(_ string, _ uuid.UUID, _ string) {}

func (mock *freezeTrackerMock) TrackViolationFrozenUnpaidInvoice(_ string, _ uuid.UUID, _ string) {}
