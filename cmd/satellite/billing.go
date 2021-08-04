// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb"
)

func runBillingCmd(ctx context.Context, cmdFunc func(context.Context, *stripecoinpayments.Service, satellite.DB) error) error {
	// Open SatelliteDB for the Payment Service
	logger := zap.L()
	db, err := satellitedb.Open(ctx, logger.Named("db"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-billing"})
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	payments, err := setupPayments(logger, db)
	if err != nil {
		return err
	}

	return cmdFunc(ctx, payments, db)
}

func setupPayments(log *zap.Logger, db satellite.DB) (*stripecoinpayments.Service, error) {
	pc := runCfg.Payments

	var stripeClient stripecoinpayments.StripeClient
	switch pc.Provider {
	default:
		stripeClient = stripecoinpayments.NewStripeMock(
			storj.NodeID{},
			db.StripeCoinPayments().Customers(),
			db.Console().Users(),
		)
	case "stripecoinpayments":
		stripeClient = stripecoinpayments.NewStripeClient(log, pc.StripeCoinPayments)
	}

	return stripecoinpayments.NewService(
		log.Named("payments.stripe:service"),
		stripeClient,
		pc.StripeCoinPayments,
		db.StripeCoinPayments(),
		db.Console().Projects(),
		db.ProjectAccounting(),
		pc.StorageTBPrice,
		pc.EgressTBPrice,
		pc.ObjectPrice,
		pc.BonusRate,
		pc.CouponValue,
		pc.CouponDuration.IntPointer(),
		pc.CouponProjectLimit,
		pc.MinCoinPayment)
}

// parseBillingPeriodFromString parses provided date string and returns corresponding time.Time.
func parseBillingPeriod(s string) (time.Time, error) {
	values := strings.Split(s, "/")

	if len(values) != 2 {
		return time.Time{}, errs.New("invalid date format %s, use mm/yyyy", s)
	}

	month, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return time.Time{}, errs.New("can not parse month: %v", err)
	}
	year, err := strconv.ParseInt(values[1], 10, 64)
	if err != nil {
		return time.Time{}, errs.New("can not parse year: %v", err)
	}

	date := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	if date.Year() != int(year) || date.Month() != time.Month(month) || date.Day() != 1 {
		return date, errs.New("dates mismatch have %s result %s", s, date)
	}

	return date, nil
}

// userData contains the uuid and email of a satellite user.
type userData struct {
	ID    uuid.UUID
	Email string
}

// generateStripeCustomers creates missing stripe-customers for users in our database.
func generateStripeCustomers(ctx context.Context) (err error) {
	return runBillingCmd(ctx, func(ctx context.Context, payments *stripecoinpayments.Service, db satellite.DB) error {
		accounts := payments.Accounts()

		cusDB := db.StripeCoinPayments().Customers().Raw()

		rows, err := cusDB.Query(ctx, "SELECT id, email FROM users WHERE id NOT IN (SELECT user_id from stripe_customers) AND users.status=1")
		if err != nil {
			return err
		}
		defer func() {
			err = errs.Combine(err, rows.Close())
		}()

		var n int64
		for rows.Next() {
			n++
			var user userData
			err := rows.Scan(&user.ID, &user.Email)
			if err != nil {
				return err
			}

			err = accounts.Setup(ctx, user.ID, user.Email)
			if err != nil {
				return err
			}

		}

		zap.L().Info("Ensured Stripe-Customer", zap.Int64("created", n))

		return err
	})
}

// checkPaidTier ensures that all customers with a credit card are in the paid tier.
func checkPaidTier(ctx context.Context) (err error) {
	usageLimitsConfig := runCfg.Console.UsageLimits

	fmt.Println("This command will do the following:\nFor every user who has added a credit card and is not already in the paid tier:")
	fmt.Printf("Move this user to the paid tier and change their current project limits to:\n\tStorage: %s\n\tBandwidth: %s\n", usageLimitsConfig.Storage.Paid.String(), usageLimitsConfig.Bandwidth.Paid.String())
	fmt.Printf("Do you really want to run this command? (confirm with 'yes') ")

	var confirm string
	n, err := fmt.Scanln(&confirm)
	if err != nil {
		if n != 0 {
			return err
		}
		// fmt.Scanln cannot handle empty input
		confirm = "n"
	}

	if strings.ToLower(confirm) != "yes" {
		fmt.Println("Aborted - no users or projects have been modified")
		return nil
	}

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripecoinpayments.Service, db satellite.DB) error {
		customers := db.StripeCoinPayments().Customers()
		creditCards := payments.Accounts().CreditCards()
		users := db.Console().Users()
		projects := db.Console().Projects()

		usersUpgraded := 0
		projectsUpgraded := 0
		failedUsers := make(map[uuid.UUID]bool)
		morePages := true
		nextOffset := int64(0)
		listingLimit := 100
		end := time.Now()
		for morePages {
			if err = ctx.Err(); err != nil {
				return err
			}

			customersPage, err := customers.List(ctx, nextOffset, listingLimit, end)
			if err != nil {
				return err
			}
			morePages = customersPage.Next
			nextOffset = customersPage.NextOffset

			for _, c := range customersPage.Customers {
				user, err := users.Get(ctx, c.UserID)
				if err != nil {
					fmt.Printf("Couldn't find user in DB; skipping: %v\n", err)
					continue
				}
				if user.PaidTier {
					// already in paid tier; go to next customer
					continue
				}
				cards, err := creditCards.List(ctx, user.ID)
				if err != nil {
					fmt.Printf("Couldn't list user's credit cards in Stripe; skipping: %v\n", err)
					continue
				}
				if len(cards) == 0 {
					// no card added, so no paid tier; go to next customer
					continue
				}

				// convert user to paid tier
				err = users.UpdatePaidTier(ctx, user.ID, true)
				if err != nil {
					return err
				}
				usersUpgraded++

				// increase limits of existing projects to paid tier
				userProjects, err := projects.GetOwn(ctx, user.ID)
				if err != nil {
					failedUsers[user.ID] = true
					fmt.Printf("Error getting user's projects; skipping: %v\n", err)
					continue
				}
				for _, project := range userProjects {
					if project.StorageLimit == nil || *project.StorageLimit < usageLimitsConfig.Storage.Paid {
						project.StorageLimit = new(memory.Size)
						*project.StorageLimit = usageLimitsConfig.Storage.Paid
					}
					if project.BandwidthLimit == nil || *project.BandwidthLimit < usageLimitsConfig.Bandwidth.Paid {
						project.BandwidthLimit = new(memory.Size)
						*project.BandwidthLimit = usageLimitsConfig.Bandwidth.Paid
					}
					err = projects.Update(ctx, &project)
					if err != nil {
						failedUsers[user.ID] = true
						fmt.Printf("Error updating user's project; skipping: %v\n", err)
						continue
					}
					projectsUpgraded++
				}
			}
		}
		fmt.Printf("Finished. Upgraded %d users and %d projects.\n", usersUpgraded, projectsUpgraded)

		if len(failedUsers) > 0 {
			fmt.Println("Failed to upgrade some users' projects to paid tier:")
			for id := range failedUsers {
				fmt.Println(id.String())
			}
		}

		return nil
	})
}
