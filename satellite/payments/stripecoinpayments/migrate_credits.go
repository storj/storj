// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"encoding/json"
	"time"

	"github.com/stripe/stripe-go"
	"go.uber.org/zap"

	"storj.io/common/uuid"
)

type migrationStats struct {
	processedCustomers           int
	customersWithCreditRecords   int
	customersWithPositiveBalance int
	migratedBalanceAmount        int64
	migratedHistoryAmount        int64
	migratedCreditRecords        int
}

// MigrateCredits migrates credits from Satellite DB to Stripe balance.
func (service *Service) MigrateCredits(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now().UTC()
	stats := &migrationStats{}

	cusPage, err := service.db.Customers().List(ctx, 0, service.listingLimit, now)
	if err != nil {
		return Error.Wrap(err)
	}

	for _, cus := range cusPage.Customers {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		if err = service.migrateCredits(ctx, cus, stats); err != nil {
			return Error.Wrap(err)
		}
	}

	for cusPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		cusPage, err = service.db.Customers().List(ctx, cusPage.NextOffset, service.listingLimit, now)
		if err != nil {
			return Error.Wrap(err)
		}

		for _, cus := range cusPage.Customers {
			if err = ctx.Err(); err != nil {
				return Error.Wrap(err)
			}

			if err = service.migrateCredits(ctx, cus, stats); err != nil {
				return Error.Wrap(err)
			}
		}
	}

	service.log.Info("Migration complete.",
		zap.Int("Processed Customers", stats.processedCustomers),
		zap.Int("Customers With Credit Records", stats.customersWithCreditRecords),
		zap.Int("Customers With Positive Credit Balance", stats.customersWithPositiveBalance),
		zap.Int64("Balance Amount in Cents Migrated to Stripe", stats.migratedBalanceAmount),
		zap.Int("Credit History Records Migrated to Stripe", stats.migratedCreditRecords),
		zap.Int64("Credit History Amount in Cents Migrated to Stripe", stats.migratedHistoryAmount),
	)

	return nil
}

func (service *Service) migrateCredits(ctx context.Context, customer Customer, stats *migrationStats) error {
	stats.processedCustomers++

	err := service.migrateCreditsBalance(ctx, customer, stats)
	if err != nil {
		return Error.Wrap(err)
	}

	err = service.migrateCreditsHistory(ctx, customer, stats)
	return Error.Wrap(err)
}

func (service *Service) migrateCreditsBalance(ctx context.Context, customer Customer, stats *migrationStats) error {
	balance, err := service.db.Credits().Balance(ctx, customer.UserID)
	if err != nil {
		return Error.Wrap(err)
	}

	if balance <= 0 {
		return nil
	}

	stats.customersWithPositiveBalance++

	service.log.Info("Found positive credit balance.",
		zap.Int64("Balance", balance),
		zap.Stringer("User ID", customer.UserID),
		zap.String("Customer ID", customer.ID),
	)

	// Check for Stripe balance transactions created from previous failed attempt
	var txDone bool
	it := service.stripeClient.CustomerBalanceTransactions().List(&stripe.CustomerBalanceTransactionListParams{Customer: stripe.String(customer.ID)})
	for it.Next() {
		cbt := it.CustomerBalanceTransaction()

		if cbt.Type != stripe.CustomerBalanceTransactionTypeAdjustment {
			continue
		}

		if cbt.Description != StripeMigratedDepositBonusTransactionDescription {
			continue
		}

		if cbt.Amount != -balance {
			return Error.New("amount mismatch in found balance transaction, want: %d, got: %d", -balance, cbt.Amount)
		}

		service.log.Warn("Found balance transaction in Stripe from previous attempt.",
			zap.Int64("Amount", cbt.Amount),
			zap.Time("Created At", time.Unix(cbt.Created, 0)),
			zap.Stringer("User ID", customer.UserID),
			zap.String("Customer ID", customer.ID),
		)

		txDone = true
	}

	// Add the unspent credits to Stripe balance
	if !txDone {
		params := &stripe.CustomerBalanceTransactionParams{
			Amount:      stripe.Int64(-balance),
			Customer:    stripe.String(customer.ID),
			Currency:    stripe.String(string(stripe.CurrencyUSD)),
			Description: stripe.String(StripeMigratedDepositBonusTransactionDescription),
		}

		stats.migratedBalanceAmount += balance

		service.log.Info("Crediting Stripe balance.",
			zap.Int64("Amount", *params.Amount),
			zap.Stringer("User ID", customer.UserID),
			zap.String("Customer ID", customer.ID),
		)

		_, err = service.stripeClient.CustomerBalanceTransactions().New(params)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	// Clear the credits balance in the satellite DB
	creditSpendingID, err := uuid.New()
	if err != nil {
		return Error.Wrap(err)
	}

	service.log.Info("Issuing a credit spending to clear balance in satellite DB.",
		zap.Int64("Amount", balance),
		zap.Stringer("User ID", customer.UserID),
		zap.String("Customer ID", customer.ID),
	)

	err = service.db.Credits().InsertCreditsSpending(ctx, CreditsSpending{
		ID:     creditSpendingID,
		Amount: balance,
		UserID: customer.UserID,
		Status: CreditsSpendingStatusApplied,
		Period: time.Now().UTC(),
	})
	return Error.Wrap(err)
}

func (service *Service) migrateCreditsHistory(ctx context.Context, customer Customer, stats *migrationStats) error {
	credits, err := service.db.Credits().ListCredits(ctx, customer.UserID)
	if err != nil {
		return Error.Wrap(err)
	}

	if len(credits) == 0 {
		return nil
	}

	stats.customersWithCreditRecords++

	service.log.Info("Found credit records in satellite DB.",
		zap.Int("Count", len(credits)),
		zap.Stringer("User ID", customer.UserID),
		zap.String("Customer ID", customer.ID),
	)

	stripeCustomer, err := service.stripeClient.Customers().Get(customer.ID, nil)
	if err != nil {
		return Error.Wrap(err)
	}

	metadata := stripeCustomer.Metadata

	for _, credit := range credits {
		metadataKey := "credit_" + credit.TransactionID.String()
		_, ok := metadata[metadataKey]
		if ok {
			// the credit record already exist in metadata
			continue
		}

		b, err := json.Marshal(credit)
		if err != nil {
			return Error.Wrap(err)
		}

		metadataValue := string(b)

		stats.migratedCreditRecords++
		stats.migratedHistoryAmount += credit.Amount

		service.log.Info("Adding credit record to the metadata of Stripe customer.",
			zap.String("Key", metadataKey),
			zap.String("Value", metadataValue),
			zap.Stringer("User ID", customer.UserID),
			zap.String("Customer ID", customer.ID),
		)

		if metadata == nil {
			metadata = make(map[string]string)
		}
		metadata[metadataKey] = metadataValue
	}

	params := stripe.CustomerParams{}
	params.Metadata = metadata

	_, err = service.stripeClient.Customers().Update(customer.ID, &params)
	return Error.Wrap(err)
}
