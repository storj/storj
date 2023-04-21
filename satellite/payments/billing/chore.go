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
}

// NewChore creates new chore.
func NewChore(log *zap.Logger, paymentTypes []PaymentType, transactionsDB TransactionsDB, interval time.Duration, disableLoop bool, bonusRate int64) *Chore {
	return &Chore{
		log:              log,
		paymentTypes:     paymentTypes,
		transactionsDB:   transactionsDB,
		TransactionCycle: sync2.NewCycle(interval),
		disableLoop:      disableLoop,
		bonusRate:        bonusRate,
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
			lastTransactionTime, lastTransactionMetadata, err := chore.transactionsDB.LastTransaction(ctx, paymentType.Source(), paymentType.Type())
			if err != nil && !errs.Is(err, ErrNoTransactions) {
				chore.log.Error("unable to determine timestamp of last transaction", zap.Error(ChoreErr.Wrap(err)))
				continue
			}
			transactions, err := paymentType.GetNewTransactions(ctx, lastTransactionTime, lastTransactionMetadata)
			if err != nil {
				chore.log.Error("unable to get new billing transactions", zap.Error(ChoreErr.Wrap(err)))
				continue
			}
			for _, transaction := range transactions {
				if bonus, ok := prepareBonusTransaction(chore.bonusRate, paymentType.Source(), transaction); ok {
					_, err = chore.transactionsDB.Insert(ctx, transaction, bonus)
				} else {
					_, err = chore.transactionsDB.Insert(ctx, transaction)
				}
				if err != nil {
					chore.log.Error("error storing transaction to db", zap.Error(ChoreErr.Wrap(err)))
					// we need to halt storing transactions if one fails, so that it can be tried again on the next loop.
					break
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
