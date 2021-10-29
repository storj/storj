// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/sync2"
)

// ErrChore is stripecoinpayments clearing loop chore error class.
var ErrChore = errs.Class("stripecoinpayments chore")

// Chore runs clearing process of reconciling transactions deposits,
// customer balance, invoices and usages.
//
// architecture: Chore
type Chore struct {
	log                 *zap.Logger
	service             *Service
	TransactionCycle    *sync2.Cycle
	AccountBalanceCycle *sync2.Cycle

	// temporary! remove once all gob-encoded big.Float records are gone from DBs on all satellites:
	TransactionMigrationCycle    *sync2.Cycle
	ConversionRateMigrationCycle *sync2.Cycle
	migrationBatchSize           int
}

// NewChore creates new clearing loop chore.
// TODO: uncomment new interval when coupons will be finished.
func NewChore(log *zap.Logger, service *Service, txInterval, accBalanceInterval, migrationBatchInterval time.Duration, migrationBatchSize int) *Chore {
	return &Chore{
		log:                          log,
		service:                      service,
		TransactionCycle:             sync2.NewCycle(txInterval),
		AccountBalanceCycle:          sync2.NewCycle(accBalanceInterval),
		TransactionMigrationCycle:    sync2.NewCycle(migrationBatchInterval),
		ConversionRateMigrationCycle: sync2.NewCycle(migrationBatchInterval),
		migrationBatchSize:           migrationBatchSize,
	}
}

// Run runs all clearing related cycles.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group

	chore.TransactionCycle.Start(ctx, &group,
		func(ctx context.Context) error {
			chore.log.Info("running transactions update cycle")

			if err := chore.service.updateTransactionsLoop(ctx); err != nil {
				chore.log.Error("transaction update cycle failed", zap.Error(ErrChore.Wrap(err)))
			}

			return nil
		},
	)
	chore.AccountBalanceCycle.Start(ctx, &group,
		func(ctx context.Context) error {
			chore.log.Info("running account balance update cycle")

			if err := chore.service.updateAccountBalanceLoop(ctx); err != nil {
				chore.log.Error("account balance update cycle failed", zap.Error(ErrChore.Wrap(err)))
			}

			return nil
		},
	)

	var transactionMigrationNextRange string
	var transactionMigrationDone bool
	chore.TransactionMigrationCycle.Start(ctx, &group,
		func(ctx context.Context) (err error) {
			if transactionMigrationDone {
				mon.Event("coinpayments_transactions_gob_encoded_big_float_migration_done")
				return nil
			}
			var migrated int
			migrated, transactionMigrationNextRange, err = chore.service.db.Transactions().MigrateGobFloatTransactionRecords(ctx, transactionMigrationNextRange, chore.migrationBatchSize)
			mon.Meter("coinpayments_transactions_gob_encoded_big_float_rows_migrated").Mark(migrated)
			if transactionMigrationNextRange == "" {
				transactionMigrationDone = true
			}
			if err != nil {
				if !errs2.IsCanceled(err) {
					chore.log.Error("gob-encoded big.Float transaction migration chore failed", zap.Error(ErrChore.Wrap(err)))
				}
				return err
			}
			return nil
		},
	)

	var conversionRateMigrationNextRange string
	var conversionRateMigrationDone bool
	chore.ConversionRateMigrationCycle.Start(ctx, &group,
		func(ctx context.Context) (err error) {
			if conversionRateMigrationDone {
				mon.Event("stripecoinpayments_tx_conversion_rates_gob_encoded_big_float_migration_done")
				return nil
			}
			var migrated int
			migrated, conversionRateMigrationNextRange, err = chore.service.db.Transactions().MigrateGobFloatConversionRateRecords(ctx, conversionRateMigrationNextRange, chore.migrationBatchSize)
			mon.Meter("stripecoinpayments_tx_conversion_rates_gob_encoded_big_float_rows_migrated").Mark(migrated)
			if conversionRateMigrationNextRange == "" {
				conversionRateMigrationDone = true
			}
			if err != nil {
				if !errs2.IsCanceled(err) {
					chore.log.Error("gob-encoded big.Float conversion rate migration chore failed", zap.Error(ErrChore.Wrap(err)))
				}
				return err
			}
			return nil
		},
	)

	return ErrChore.Wrap(group.Wait())
}

// Close closes all underlying resources.
func (chore *Chore) Close() (err error) {
	defer mon.Task()(nil)(&err)

	chore.TransactionCycle.Close()
	chore.AccountBalanceCycle.Close()
	chore.TransactionMigrationCycle.Close()
	chore.ConversionRateMigrationCycle.Close()
	return nil
}
