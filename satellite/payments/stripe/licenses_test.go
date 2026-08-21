// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"bytes"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	stripeSDK "github.com/stripe/stripe-go/v81"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/stripe"
)

const (
	// licenseProductID is a product with license fee.
	licenseProductID  = 7
	licenseProduct2ID = 9
	// usageOnlyProductID is a product without a license fee.
	usageOnlyProductID = 8

	licenseSeatSKU  = "OM-STORJOS-SEAT"
	licenseSeat2SKU = "OM-ANYCLOUD-SEAT"
)

func TestLicenseSeatBilling(t *testing.T) {
	var products paymentsconfig.ProductPriceOverrides
	products.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
		licenseProductID: {
			Name:          "Object Mount (Storj OS)",
			LicenseFee:    "29.00",
			LicenseFeeSKU: licenseSeatSKU,
		},
		licenseProduct2ID: {
			Name:          "Object Mount (Any Cloud)",
			LicenseFee:    "39.00",
			LicenseFeeSKU: licenseSeat2SKU,
		},
		usageOnlyProductID: {
			Name: "Usage Only",
			ProjectUsagePrice: paymentsconfig.ProjectUsagePrice{
				StorageTB: "4", EgressTB: "7", Segment: "0.0000088",
			},
		},
	})

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.Products = products
				config.Entitlements.Enabled = true
				config.Payments.StripeCoinPayments.PopulateLicenseInvoiceLineItem = true
				config.Payments.StripeCoinPayments.SkuEnabled = true
				config.Payments.StripeCoinPayments.InvItemSKUInDescription = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		stripeService := sat.API.Payments.StripeService

		t.Run("line items", func(t *testing.T) {
			// A fixed 31-day month, so the proration fractions are exact.
			start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
			userID := testrand.UUID()

			t.Run("full month", func(t *testing.T) {
				items := stripeService.InvoiceItemsFromLicenses([]entitlements.AccountLicense{{
					Type:      entitlements.OMLicenseType,
					ProductID: licenseProductID,
					Count:     5,
					StartsAt:  start,
					ExpiresAt: end.AddDate(1, 0, 0),
				}}, userID, start, end)

				require.Len(t, items, 1)
				require.EqualValues(t, 5, *items[0].Quantity)
				require.EqualValues(t, 2900, *items[0].UnitAmount)
				require.Equal(t, "Object Mount (Storj OS) - Seats", *items[0].Description)
				require.Equal(t, licenseSeatSKU, items[0].Metadata["SKU"])
				require.Equal(t, licenseSeatSKU, items[0].Metadata["ItemCode"])
			})

			t.Run("no item produced", func(t *testing.T) {
				for _, tt := range []struct {
					name   string
					mutate func(*entitlements.AccountLicense)
				}{
					{"free license", func(l *entitlements.AccountLicense) { l.ProductID = 0 }},
					{"unknown product", func(l *entitlements.AccountLicense) { l.ProductID = 9999 }},
					{"product without a license fee", func(l *entitlements.AccountLicense) {
						l.ProductID = usageOnlyProductID
					}},
					{"revoked before the period", func(l *entitlements.AccountLicense) {
						l.StartsAt = start.AddDate(0, -2, 0)
						l.RevokedAt = start.AddDate(0, -1, 0)
					}},
					{"zero seats", func(l *entitlements.AccountLicense) { l.Count = 0 }},
				} {
					license := entitlements.AccountLicense{
						Type:      entitlements.OMLicenseType,
						ProductID: licenseProductID,
						Count:     3,
						StartsAt:  start,
						ExpiresAt: end.AddDate(1, 0, 0),
					}
					tt.mutate(&license)

					items := stripeService.InvoiceItemsFromLicenses(
						[]entitlements.AccountLicense{license}, userID, start, end)

					require.Empty(t, items, tt.name)
				}
			})

			t.Run("revoked mid-period is billed up to the revocation", func(t *testing.T) {
				items := stripeService.InvoiceItemsFromLicenses([]entitlements.AccountLicense{{
					Type:      entitlements.OMLicenseType,
					ProductID: licenseProductID,
					Count:     4,
					StartsAt:  start,
					ExpiresAt: end.AddDate(1, 0, 0),
					RevokedAt: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC),
				}}, userID, start, end)

				require.Len(t, items, 1)
				require.EqualValues(t, 4, *items[0].Quantity)
				// 9 of 31 days: 2900 * 9 / 31 = 841.94 cents, rounded to the cent.
				require.EqualValues(t, 842, *items[0].UnitAmount)
				require.Equal(t, "Object Mount (Storj OS) - Seats (9/31 days)", *items[0].Description)
			})

			t.Run("same product and rate aggregates into one item", func(t *testing.T) {
				license := entitlements.AccountLicense{
					Type:      entitlements.OMLicenseType,
					ProductID: licenseProductID,
					Count:     2,
					StartsAt:  start,
					ExpiresAt: end.AddDate(1, 0, 0),
				}
				other := license
				other.Count = 3
				other.PublicID = testrand.UUID().String()

				items := stripeService.InvoiceItemsFromLicenses(
					[]entitlements.AccountLicense{license, other}, userID, start, end)

				require.Len(t, items, 1)
				require.EqualValues(t, 5, *items[0].Quantity)
			})

			t.Run("different start dates produce separate items", func(t *testing.T) {
				full := entitlements.AccountLicense{
					Type:      entitlements.OMLicenseType,
					ProductID: licenseProductID,
					Count:     2,
					StartsAt:  start,
					ExpiresAt: end.AddDate(1, 0, 0),
				}
				partial := full
				partial.Count = 3
				partial.StartsAt = time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)

				items := stripeService.InvoiceItemsFromLicenses(
					[]entitlements.AccountLicense{full, partial}, userID, start, end)

				require.Len(t, items, 2)

				// Sorted by billed days ascending, so the prorated item comes first.
				require.EqualValues(t, 3, *items[0].Quantity)
				require.Equal(t, "Object Mount (Storj OS) - Seats (11/31 days)", *items[0].Description)
				require.EqualValues(t, 2, *items[1].Quantity)
				require.Equal(t, "Object Mount (Storj OS) - Seats", *items[1].Description)

				require.NotEqual(t, *items[0].Params.IdempotencyKey, *items[1].Params.IdempotencyKey)
			})

			t.Run("separate products produce separate items", func(t *testing.T) {
				base := entitlements.AccountLicense{
					Type:      entitlements.OMLicenseType,
					StartsAt:  start,
					ExpiresAt: end.AddDate(1, 0, 0),
				}
				first := base
				first.ProductID = licenseProductID
				first.Count = 3
				second := base
				second.ProductID = licenseProduct2ID
				second.Count = 2

				// Passed in descending product order to show the output is sorted.
				items := stripeService.InvoiceItemsFromLicenses(
					[]entitlements.AccountLicense{second, first}, userID, start, end)

				require.Len(t, items, 2)

				require.Equal(t, "Object Mount (Storj OS) - Seats", *items[0].Description)
				require.EqualValues(t, 3, *items[0].Quantity)
				require.EqualValues(t, 2900, *items[0].UnitAmount)
				require.Equal(t, licenseSeatSKU, items[0].Metadata["SKU"])

				require.Equal(t, "Object Mount (Any Cloud) - Seats", *items[1].Description)
				require.EqualValues(t, 2, *items[1].Quantity)
				require.EqualValues(t, 3900, *items[1].UnitAmount)
				require.Equal(t, licenseSeat2SKU, items[1].Metadata["SKU"])

				require.NotEqual(t, *items[0].Params.IdempotencyKey, *items[1].Params.IdempotencyKey)
			})

			t.Run("prorated seat price is rounded to the cent", func(t *testing.T) {
				for _, tt := range []struct {
					startDay int
					cents    int64
				}{
					// 2900 * billedDays / 31, rounded half up.
					{startDay: 2, cents: 2806}, // 30 days: 2806.45, rounds down
					{startDay: 31, cents: 94},  // 1 day: 93.55, rounds up
				} {
					items := stripeService.InvoiceItemsFromLicenses([]entitlements.AccountLicense{{
						Type:      entitlements.OMLicenseType,
						ProductID: licenseProductID,
						Count:     3,
						StartsAt:  time.Date(2026, 7, tt.startDay, 0, 0, 0, 0, time.UTC),
						ExpiresAt: end.AddDate(1, 0, 0),
					}}, userID, start, end)

					require.Len(t, items, 1, "start day %d", tt.startDay)
					require.EqualValues(t, tt.cents, *items[0].UnitAmount, "start day %d", tt.startDay)
					// Never a fractional-cent amount, which Stripe bills differently.
					require.Nil(t, items[0].UnitAmountDecimal, "start day %d", tt.startDay)
				}
			})

			t.Run("sku config", func(t *testing.T) {
				defer func() {
					stripeService.TestSetSkuEnabled(true)
					stripeService.TestSetInvItemSKUInDescription(false)
				}()

				for _, tt := range []struct {
					name          string
					skuEnabled    bool
					inDescription bool
					wantSKU       string
					wantDesc      string
					wantProrated  string
				}{
					{
						name:         "sku disabled",
						wantDesc:     "Object Mount (Storj OS) - Seats",
						wantProrated: "Object Mount (Storj OS) - Seats (11/31 days)",
					},
					{
						name:          "sku in description",
						skuEnabled:    true,
						inDescription: true,
						wantSKU:       licenseSeatSKU,
						wantDesc:      "Object Mount (Storj OS) - Seats - " + licenseSeatSKU,
						// The day fraction stays ahead of the SKU when prorated.
						wantProrated: "Object Mount (Storj OS) - Seats (11/31 days) - " + licenseSeatSKU,
					},
				} {
					stripeService.TestSetSkuEnabled(tt.skuEnabled)
					stripeService.TestSetInvItemSKUInDescription(tt.inDescription)

					license := entitlements.AccountLicense{
						Type:      entitlements.OMLicenseType,
						ProductID: licenseProductID,
						Count:     2,
						StartsAt:  start,
						ExpiresAt: end.AddDate(1, 0, 0),
					}

					items := stripeService.InvoiceItemsFromLicenses(
						[]entitlements.AccountLicense{license}, userID, start, end)
					require.Len(t, items, 1, tt.name)
					require.Equal(t, tt.wantDesc, *items[0].Description, tt.name)
					require.Equal(t, tt.wantSKU, items[0].Metadata["SKU"], tt.name)

					license.StartsAt = time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
					items = stripeService.InvoiceItemsFromLicenses(
						[]entitlements.AccountLicense{license}, userID, start, end)
					require.Len(t, items, 1, tt.name)
					require.Equal(t, tt.wantProrated, *items[0].Description, tt.name)
				}
			})
		})

		// The customer walk only lists customers created before the period end, so this
		// period has to be the month ahead of now; SetNow then makes it read as already
		// ended. Every subtest works on its own account, so the satellite-wide walk each
		// run performs does not make them interfere.
		t.Run("billing flow", func(t *testing.T) {
			stripeClient := sat.API.Payments.StripeClient
			licenseRecords := sat.DB.StripeCoinPayments().LicenseRecords()
			// Following the convention in service_test.go, the period is the month
			// after the current one, so the accounts created below predate its end.
			now := time.Now()
			period := time.Date(now.Year(), now.Month()+1, 20, 0, 0, 0, 0, time.UTC)
			start := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(period.Year(), period.Month()+1, 1, 0, 0, 0, 0, time.UTC)

			// licenseUser creates an account of the given kind holding the given
			// licenses, and returns it with its stripe customer ID. It takes t so a
			// failure lands on the subtest that asked for the account.
			licenseUser := func(t *testing.T, email string, kind console.UserKind, licenses ...entitlements.AccountLicense) (*console.User, string) {
				user, err := sat.AddUser(ctx, console.CreateUser{
					FullName: email,
					Email:    email,
					Kind:     kind,
				}, 1)
				require.NoError(t, err)

				cusID, err := sat.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
				require.NoError(t, err)

				if len(licenses) > 0 {
					require.NoError(t, sat.API.Entitlements.Service.Licenses().Set(ctx, user.ID,
						entitlements.AccountLicenses{Licenses: licenses}))
				}

				return user, cusID
			}

			stripeService.SetNow(func() time.Time { return end })

			fullPeriodLicense := func(seats int) entitlements.AccountLicense {
				return entitlements.AccountLicense{
					Type:      entitlements.OMLicenseType,
					ProductID: licenseProductID,
					Count:     seats,
					StartsAt:  start,
					ExpiresAt: end.AddDate(1, 0, 0),
				}
			}

			t.Run("a billable license is charged", func(t *testing.T) {
				// A free license alongside the paid one; only the paid one is billed.
				_, cusID := licenseUser(t, "license-user@mail.test", console.PaidUser,
					entitlements.AccountLicense{
						Type:      entitlements.OMLicenseType,
						Count:     2,
						StartsAt:  start,
						ExpiresAt: end.AddDate(100, 0, 0),
					},
					fullPeriodLicense(5),
				)

				require.NoError(t, stripeService.CreateLicenseInvoiceItems(ctx, period))

				items := getCustomerInvoiceItems(ctx, stripeClient, cusID)
				require.Len(t, items, 1)
				require.Equal(t, "Object Mount (Storj OS) - Seats", items[0].Description)
				require.EqualValues(t, 5, items[0].Quantity)
				require.EqualValues(t, 2900, items[0].UnitAmount)
				require.Equal(t, licenseSeatSKU, items[0].Metadata["SKU"])

				// The idempotency key must prevent a second seat charge.
				require.NoError(t, stripeService.CreateLicenseInvoiceItems(ctx, period))
				again := getCustomerInvoiceItems(ctx, stripeClient, cusID)
				require.Len(t, again, 1)
				require.Equal(t, items[0].ID, again[0].ID)
			})

			t.Run("future period is rejected", func(t *testing.T) {
				require.Error(t, stripeService.CreateLicenseInvoiceItems(ctx, period.AddDate(0, 1, 0)))
			})

			t.Run("user without licenses gets no items", func(t *testing.T) {
				_, cusID := licenseUser(t, "no-license-user@mail.test", console.PaidUser)

				require.NoError(t, stripeService.CreateLicenseInvoiceItems(ctx, period))
				require.Empty(t, getCustomerInvoiceItems(ctx, stripeClient, cusID))
			})

			t.Run("billing exempt account is not charged", func(t *testing.T) {
				user, cusID := licenseUser(t, "billing-exempt-license@mail.test",
					console.FreeUser, fullPeriodLicense(5))
				require.True(t, user.IsBillingExempt())

				require.NoError(t, stripeService.CreateLicenseInvoiceItems(ctx, period))

				require.Empty(t, getCustomerInvoiceItems(ctx, stripeClient, cusID))
				unapplied, err := licenseRecords.ListUnappliedByUser(ctx, user.ID, start, end)
				require.NoError(t, err)
				require.Empty(t, unapplied)
			})

			t.Run("entitlements disabled is a no-op", func(t *testing.T) {
				_, cusID := licenseUser(t, "disabled-entitlements@mail.test",
					console.PaidUser, fullPeriodLicense(5))

				// The licenses are set through the entitlements service either way; it is
				// the payments side of the flag that decides whether they are billed.
				stripeService.TestSetEntitlementsEnabled(false)
				defer stripeService.TestSetEntitlementsEnabled(true)

				require.NoError(t, stripeService.CreateLicenseInvoiceItems(ctx, period))
				require.Empty(t, getCustomerInvoiceItems(ctx, stripeClient, cusID))
			})

			t.Run("line item population disabled is a no-op", func(t *testing.T) {
				_, cusID := licenseUser(t, "disabled-line-items@mail.test",
					console.PaidUser, fullPeriodLicense(5))

				stripeService.TestSetPopulateLicenseInvoiceLineItem(false)
				defer stripeService.TestSetPopulateLicenseInvoiceLineItem(true)

				require.NoError(t, stripeService.CreateLicenseInvoiceItems(ctx, period))
				require.Empty(t, getCustomerInvoiceItems(ctx, stripeClient, cusID))
			})

			t.Run("an unapplied record is retried", func(t *testing.T) {
				user, cusID := licenseUser(t, "license-retry@mail.test", console.PaidUser)

				require.NoError(t, licenseRecords.Create(ctx, []stripe.CreateLicenseRecord{{
					UserID:          user.ID,
					ProductID:       licenseProductID,
					Seats:           4,
					BilledDays:      11,
					DaysInPeriod:    31,
					UnitAmountCents: 1029,
				}}, start, end))

				require.NoError(t, stripeService.CreateLicenseInvoiceItems(ctx, period))

				items := getCustomerInvoiceItems(ctx, stripeClient, cusID)
				require.Len(t, items, 1)
				require.Equal(t, "Object Mount (Storj OS) - Seats (11/31 days)", items[0].Description)
				require.EqualValues(t, 4, items[0].Quantity)
				require.EqualValues(t, 1029, items[0].UnitAmount)

				unapplied, err := licenseRecords.ListUnappliedByUser(ctx, user.ID, start, end)
				require.NoError(t, err)
				require.Empty(t, unapplied)
			})

			// The license record, not the Stripe idempotency key, is the durable guard
			// against double billing: Stripe forgets a key after 24 hours, so the step has
			// to stay safe once the key is gone.
			t.Run("the record guards against double billing", func(t *testing.T) {
				user, cusID := licenseUser(t, "record-guard@mail.test",
					console.PaidUser, fullPeriodLicense(5))

				require.NoError(t, stripeService.CreateLicenseInvoiceItems(ctx, period))

				items := getCustomerInvoiceItems(ctx, stripeClient, cusID)
				require.Len(t, items, 1)
				require.EqualValues(t, 5, items[0].Quantity)

				// The charge was recorded and consumed. The license covers the whole
				// period, so it was billed for every day of it; licensePeriod is not a
				// fixed month, so derive the length.
				unapplied, err := licenseRecords.ListUnappliedByUser(ctx, user.ID, start, end)
				require.NoError(t, err)
				require.Empty(t, unapplied, "the record should have been consumed")

				daysInPeriod := int(end.Sub(start).Hours() / 24)
				require.ErrorIs(t,
					licenseRecords.Check(ctx, user.ID, licenseProductID, daysInPeriod, start, end),
					stripe.ErrLicenseRecordExists)

				// Grant more seats for the same period, then re-run with the idempotency
				// key gone. The prepared charge is what stands.
				require.NoError(t, sat.API.Entitlements.Service.Licenses().Set(ctx, user.ID,
					entitlements.AccountLicenses{Licenses: []entitlements.AccountLicense{fullPeriodLicense(50)}}))

				expirer, ok := stripeClient.(stripe.IdempotencyKeyExpirer)
				require.True(t, ok, "the stripe mock must support expiring idempotency keys")
				expirer.ExpireIdempotencyKeys()

				require.NoError(t, stripeService.CreateLicenseInvoiceItems(ctx, period))

				again := getCustomerInvoiceItems(ctx, stripeClient, cusID)
				require.Len(t, again, 1, "the license record must stop a second seat charge")
				require.Equal(t, items[0].ID, again[0].ID)
				require.EqualValues(t, 5, again[0].Quantity)
			})

			// A failed consume leaves the charge raised and the record unapplied. Past
			// the 24-hour idempotency window, only the marker on the item can say so.
			t.Run("a raised charge is not re-raised when consume did not land", func(t *testing.T) {
				user, cusID := licenseUser(t, "consume-failed@mail.test",
					console.PaidUser, fullPeriodLicense(3))

				daysInPeriod := int(end.Sub(start).Hours() / 24)
				require.NoError(t, licenseRecords.Create(ctx, []stripe.CreateLicenseRecord{{
					UserID:          user.ID,
					ProductID:       licenseProductID,
					Seats:           3,
					BilledDays:      daysInPeriod,
					DaysInPeriod:    daysInPeriod,
					UnitAmountCents: 2900,
				}}, start, end))

				unapplied, err := licenseRecords.ListUnappliedByUser(ctx, user.ID, start, end)
				require.NoError(t, err)
				require.Len(t, unapplied, 1)

				// Raise the charge as the service would, but leave the record unapplied.
				item := stripeService.InvoiceItemFromLicenseRecord(unapplied[0])
				require.NotNil(t, item)
				stripe.PrepareInvoiceItemForCustomer(ctx, item, cusID, start, end)
				raised, err := stripeClient.InvoiceItems().New(item)
				require.NoError(t, err)

				expirer, ok := stripeClient.(stripe.IdempotencyKeyExpirer)
				require.True(t, ok, "the stripe mock must support expiring idempotency keys")
				expirer.ExpireIdempotencyKeys()

				require.NoError(t, stripeService.CreateLicenseInvoiceItems(ctx, period))

				items := getCustomerInvoiceItems(ctx, stripeClient, cusID)
				require.Len(t, items, 1, "the charge must not be raised a second time")
				require.Equal(t, raised.ID, items[0].ID)

				unapplied, err = licenseRecords.ListUnappliedByUser(ctx, user.ID, start, end)
				require.NoError(t, err)
				require.Empty(t, unapplied, "the record should have been consumed")
			})

			// Usage items are raised before the seats and land on the same invoice, so
			// the two cannot each spend the whole of Stripe's per-invoice budget.
			t.Run("seat items share the invoice item budget", func(t *testing.T) {
				user, cusID := licenseUser(t, "item-budget@mail.test",
					console.PaidUser, fullPeriodLicense(5))

				// 248 is maxInvoiceItems: Stripe's 250, less the previous cycle's unpaid
				// usage and the minimum charge. Fill it the way the usage step would.
				for range 248 {
					item := &stripeSDK.InvoiceItemParams{Amount: stripeSDK.Int64(100)}
					stripe.PrepareInvoiceItemForCustomer(ctx, item, cusID, start, end)
					_, err := stripeClient.InvoiceItems().New(item)
					require.NoError(t, err)
				}

				err := stripeService.CreateLicenseInvoiceItems(ctx, period)
				require.ErrorContains(t, err, "too many invoice items")

				for _, item := range getCustomerInvoiceItems(ctx, stripeClient, cusID) {
					require.NotEqual(t, stripe.LicenseSeatItemMetadataValue,
						item.Metadata[stripe.LicenseSeatItemMetadataKey],
						"no seat item should have been raised")
				}

				// The charge stays recorded and unapplied, so leaving it would fail every
				// later run over this customer. Consume it to put the walk back in order.
				unapplied, err := licenseRecords.ListUnappliedByUser(ctx, user.ID, start, end)
				require.NoError(t, err)
				require.Len(t, unapplied, 1)
				require.NoError(t, licenseRecords.Consume(ctx, unapplied[0].ID))
			})

			// One customer's failure must not starve the rest: the step runs inside
			// GenerateInvoices ahead of CreateInvoices, so aborting the customer walk would
			// stop invoice generation for everyone behind it.
			t.Run("one failing customer does not starve the rest", func(t *testing.T) {
				// One customer per page, so a customer failing is a page failing and the
				// walk has to carry on to reach the customers behind it.
				stripeService.TestSetListingLimit(1)
				// 100 is the ListingLimit default in config.go.
				defer stripeService.TestSetListingLimit(100)

				type account struct {
					userID uuid.UUID
					cusID  string
				}
				var accounts []account
				for i := range 4 {
					user, cusID := licenseUser(t,
						fmt.Sprintf("bad-customer-%d@mail.test", i), console.PaidUser, fullPeriodLicense(2))
					accounts = append(accounts, account{userID: user.ID, cusID: cusID})
				}

				// Customers are walked in user_id order, so corrupting the lowest of these
				// puts the failure ahead of the other three.
				sort.Slice(accounts, func(i, j int) bool {
					return bytes.Compare(accounts[i].userID[:], accounts[j].userID[:]) < 0
				})
				broken, healthy := accounts[0], accounts[1:]

				// Valid JSON of the wrong shape, so the column accepts it but
				// Licenses().Get fails to unmarshal it, for this user alone. Every later
				// run would report the same failure, so the row goes away with the subtest.
				brokenScope := entitlements.ConvertUserIDToLicenseScope(broken.userID)
				_, err := sat.DB.Console().Entitlements().UpsertByScope(ctx, &entitlements.Entitlement{
					Scope:    brokenScope,
					Features: []byte(`{"licenses":"not-a-list"}`),
				})
				require.NoError(t, err)
				defer func() {
					require.NoError(t, sat.DB.Console().Entitlements().DeleteByScope(ctx, brokenScope))
				}()

				// The failure is reported...
				err = stripeService.CreateLicenseInvoiceItems(ctx, period)
				require.Error(t, err)
				require.Contains(t, err.Error(), broken.userID.String())

				// ...and it did not cost the accounts behind it their seat charges.
				require.Empty(t, getCustomerInvoiceItems(ctx, stripeClient, broken.cusID))
				for _, acct := range healthy {
					items := getCustomerInvoiceItems(ctx, stripeClient, acct.cusID)
					require.Len(t, items, 1, "customer %s was skipped", acct.cusID)
					require.EqualValues(t, 2, items[0].Quantity)
				}
			})

			t.Run("seat charges reach the invoice through GenerateInvoices", func(t *testing.T) {
				user, cusID := licenseUser(t, "generate-invoices-licenses@mail.test",
					console.PaidUser, fullPeriodLicense(3))

				require.NoError(t, stripeService.GenerateInvoices(ctx, period))

				invoice, hasInvoice := getCustomerInvoice(ctx, stripeClient, cusID)
				require.True(t, hasInvoice, "expected an invoice for the seat charge")
				require.NotNil(t, invoice)

				var seatLine *stripeSDK.InvoiceItem
				for _, line := range invoice.Lines.Data {
					if line.InvoiceItem != nil && line.InvoiceItem.Description == "Object Mount (Storj OS) - Seats" {
						seatLine = line.InvoiceItem
					}
				}
				require.NotNil(t, seatLine, "the seat item must be swept onto the invoice")
				require.EqualValues(t, 3, seatLine.Quantity)
				require.EqualValues(t, 3*2900, invoice.Total)

				// The charge was recorded and consumed as part of the same run, which also
				// shows CreateLicenseInvoiceItems runs ahead of CreateInvoices.
				unapplied, err := licenseRecords.ListUnappliedByUser(ctx, user.ID, start, end)
				require.NoError(t, err)
				require.Empty(t, unapplied)
			})

			// The minimum charge tops an invoice up so that trivial usage amounts are
			// not worth invoicing. A seat charge is not that: it is a deliberate amount,
			// and topping it up would undo the proration that decided it. Seats still
			// count towards satisfying the minimum on an invoice that also carries
			// usage, so no invoice gets larger than it was before.
			t.Run("minimum charge", func(t *testing.T) {
				const minimumCharge = int64(5000)
				// One seat at $29.00, which is below the minimum.
				const seatAmount = int64(2900)

				stripeService.TestSetMinimumChargeCfg(minimumCharge, nil)
				defer stripeService.TestSetMinimumChargeCfg(0, nil)

				// addUsageItem adds a pending item that is not a seat charge, standing
				// in for usage without setting up the whole usage pipeline.
				addUsageItem := func(t *testing.T, cusID string, amount int64) {
					_, err := stripeClient.InvoiceItems().New(&stripeSDK.InvoiceItemParams{
						Params:      stripeSDK.Params{Context: ctx},
						Customer:    stripeSDK.String(cusID),
						Amount:      stripeSDK.Int64(amount),
						Currency:    stripeSDK.String(string(stripeSDK.CurrencyUSD)),
						Description: stripeSDK.String("Usage Only - Storage (GB-Month)"),
					})
					require.NoError(t, err)
				}

				// The description lives on the nested invoice item, not on the line.
				findAdjustment := func(inv *stripeSDK.Invoice) *stripeSDK.InvoiceLineItem {
					for _, line := range inv.Lines.Data {
						if line.InvoiceItem != nil && line.InvoiceItem.Description == "Minimum charge adjustment" {
							return line
						}
					}
					return nil
				}

				t.Run("a seats only invoice is not topped up", func(t *testing.T) {
					user, cusID := licenseUser(t, "min-charge-seats-only@mail.test",
						console.PaidUser, fullPeriodLicense(1))
					require.NoError(t, stripeService.CreateLicenseInvoiceItems(ctx, period))

					invoice, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
					require.NoError(t, err)
					require.NotNil(t, invoice)
					require.Nil(t, findAdjustment(invoice), "a seats only invoice must not be topped up")
					require.EqualValues(t, seatAmount, invoice.Total)

					again, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
					require.NoError(t, err)
					require.Equal(t, invoice.ID, again.ID, "expected the draft from the first run")
					require.Nil(t, findAdjustment(again), "the re-run must not top up a seats only invoice")
					require.EqualValues(t, seatAmount, again.Total)
				})

				t.Run("an invoice with usage is still topped up", func(t *testing.T) {
					const usageAmount = int64(100)

					user, cusID := licenseUser(t, "min-charge-mixed@mail.test",
						console.PaidUser, fullPeriodLicense(1))
					require.NoError(t, stripeService.CreateLicenseInvoiceItems(ctx, period))
					addUsageItem(t, cusID, usageAmount)

					invoice, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
					require.NoError(t, err)
					// The seats count towards the minimum rather than being charged on
					// top, so the account pays the minimum and no more.
					adjustment := findAdjustment(invoice)
					require.NotNil(t, adjustment, "an invoice with usage keeps the minimum charge")
					require.EqualValues(t, minimumCharge-(seatAmount+usageAmount), adjustment.Amount)
					require.EqualValues(t, minimumCharge, invoice.Total)

					again, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
					require.NoError(t, err)
					require.Equal(t, invoice.ID, again.ID)
					require.EqualValues(t, minimumCharge, again.Total, "the minimum must not be applied twice")
				})

				t.Run("a usage only invoice is unaffected", func(t *testing.T) {
					const usageAmount = int64(200)

					user, cusID := licenseUser(t, "min-charge-usage-only@mail.test", console.PaidUser)
					addUsageItem(t, cusID, usageAmount)

					invoice, err := stripeService.CreateInvoice(ctx, cusID, user, start, end)
					require.NoError(t, err)
					adjustment := findAdjustment(invoice)
					require.NotNil(t, adjustment, "usage invoices keep the minimum charge")
					require.EqualValues(t, minimumCharge-usageAmount, adjustment.Amount)
					require.EqualValues(t, minimumCharge, invoice.Total)
				})
			})
		})
	})
}
