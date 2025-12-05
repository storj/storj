// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package billing

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
)

// Observer processes a billing transaction.
type Observer interface {
	// Process is called repeatedly for each transaction.
	Process(context.Context, Transaction) error
	// TestSetNow allows tests to have the observer act as if the current time is whatever they want.
	TestSetNow(nowFn func() time.Time)
}

// ChoreObservers holds functionality to process confirmed transactions using different types of observers.
type ChoreObservers struct {
	UpgradeUser Observer
	PayInvoices Observer
}

// ChoreErr is billing chore err class.
var ChoreErr = errs.Class("billing chore")

// Chore periodically queries for new billing transactions from payment type.
//
// architecture: Chore
type Chore struct {
	log              *zap.Logger
	paymentTypes     []PaymentType
	transactionsDB   TransactionsDB
	TransactionCycle *sync2.Cycle

	disableLoop bool
	bonusRate   int64
	observers   ChoreObservers
}

// NewChore creates new chore.
func NewChore(log *zap.Logger, paymentTypes []PaymentType, transactionsDB TransactionsDB, interval time.Duration, disableLoop bool, bonusRate int64, observers ChoreObservers) *Chore {
	return &Chore{
		log:              log,
		paymentTypes:     paymentTypes,
		transactionsDB:   transactionsDB,
		TransactionCycle: sync2.NewCycle(interval),
		disableLoop:      disableLoop,
		bonusRate:        bonusRate,
		observers:        observers,
	}
}

// Run runs billing transaction loop.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return chore.TransactionCycle.Run(ctx, func(ctx context.Context) error {
		if chore.disableLoop {
			chore.log.Debug("Skipping chore iteration as loop is disabled", zap.Bool("disableLoop", chore.disableLoop))
			return nil
		}

		for _, paymentType := range chore.paymentTypes {
			for _, source := range paymentType.Sources() {
				lastTransactionTime, lastTransactionMetadata, err := chore.transactionsDB.LastTransaction(ctx, source, paymentType.Type())
				if err != nil && !errs.Is(err, ErrNoTransactions) {
					chore.log.Error("unable to determine timestamp of last transaction", zap.Error(ChoreErr.Wrap(err)))
					continue
				}
				transactions, err := paymentType.GetNewTransactions(ctx, source, lastTransactionTime, lastTransactionMetadata)
				if err != nil {
					chore.log.Error("unable to get new billing transactions", zap.Error(ChoreErr.Wrap(err)))
					continue
				}
				for _, transaction := range transactions {
					if bonus, ok := prepareBonusTransaction(chore.bonusRate, source, transaction); ok {
						_, err = chore.transactionsDB.Insert(ctx, transaction, bonus)
					} else {
						_, err = chore.transactionsDB.Insert(ctx, transaction)
					}
					if err != nil {
						chore.log.Error("error storing transaction to db", zap.Error(ChoreErr.Wrap(err)))
						// we need to halt storing transactions if one fails, so that it can be tried again on the next loop.
						break
					}

					if chore.observers.UpgradeUser != nil {
						err = chore.observers.UpgradeUser.Process(ctx, transaction)
						if err != nil {
							// we don't want to halt storing transactions if upgrade user observer fails
							// because this chore is designed to store new transactions.
							// So auto upgrading user is a side effect which shouldn't interrupt the main process.
							chore.log.Error("error upgrading user", zap.Error(ChoreErr.Wrap(err)))
						}
					}

					if chore.observers.PayInvoices != nil {
						err = chore.observers.PayInvoices.Process(ctx, transaction)
						if err != nil {
							chore.log.Error("error paying invoices", zap.Error(ChoreErr.Wrap(err)))
						}
					}
				}
			}
		}
		return nil
	})
}

// Close closes all underlying resources.
func (chore *Chore) Close() (err error) {
	defer mon.Task()(nil)(&err)
	chore.TransactionCycle.Close()
	return nil
}

// TestSetPaymentTypes is used in tests to change the payment
// types this chore tracks.
func (chore *Chore) TestSetPaymentTypes(types []PaymentType) {
	chore.paymentTypes = types
}
