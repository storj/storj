// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/payments"
)

// ChoreErr is storjscan chore err class.
var ChoreErr = errs.Class("storjscan chore")

// Chore periodically queries for new payments from storjscan.
//
// architecture: Chore
type Chore struct {
	log              *zap.Logger
	client           *Client
	paymentsDB       PaymentsDB
	TransactionCycle *sync2.Cycle
	confirmations    int

	disableLoop bool
}

// NewChore creates new chore.
func NewChore(log *zap.Logger, client *Client, paymentsDB PaymentsDB, confirmations int, interval time.Duration, disableLoop bool) *Chore {
	return &Chore{
		log:              log,
		client:           client,
		paymentsDB:       paymentsDB,
		TransactionCycle: sync2.NewCycle(interval),
		confirmations:    confirmations,
		disableLoop:      disableLoop,
	}
}

// Run runs storjscan payment loop.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return chore.TransactionCycle.Run(ctx, func(ctx context.Context) error {
		if chore.disableLoop {
			chore.log.Debug("Skipping chore iteration as loop is disabled", zap.Bool("disableLoop", chore.disableLoop))
			return nil
		}

		from := make(map[int64]int64)
		blockNumbers, err := chore.paymentsDB.LastBlocks(ctx, payments.PaymentStatusConfirmed)
		for chainID, blockNumber := range blockNumbers {
			switch {
			case err == nil:
				from[chainID] = blockNumber + 1
			case errs.Is(err, ErrNoPayments):
				from[chainID] = 0
			default:
				chore.log.Error("error retrieving last payment", zap.Error(ChoreErr.Wrap(err)))
				return nil
			}
		}

		latestPayments, err := chore.client.AllPayments(ctx, from)
		if err != nil {
			chore.log.Error("error retrieving payments", zap.Error(ChoreErr.Wrap(err)))
			return nil
		}
		if len(latestPayments.Payments) == 0 {
			return nil
		}

		var cachedPayments []CachedPayment
		for _, header := range latestPayments.LatestBlocks {
			for _, payment := range latestPayments.Payments {
				if payment.ChainID == header.ChainID {
					var status payments.PaymentStatus
					if header.Number-payment.BlockNumber >= int64(chore.confirmations) {
						status = payments.PaymentStatusConfirmed
					} else {
						status = payments.PaymentStatusPending
					}
					chore.log.Debug("received new payments from storjscan",
						zap.Int64("Chain ID", payment.ChainID),
						zap.String("Block Hash", payment.BlockHash.Hex()),
						zap.String("Transaction Hash", payment.Transaction.Hex()),
						zap.Int64("Block Number", payment.BlockNumber),
						zap.Int("Log Index", payment.LogIndex),
						zap.String("USD Value", payment.USDValue.AsDecimal().String()))
					cachedPayments = append(cachedPayments, CachedPayment{
						ChainID:     payment.ChainID,
						From:        payment.From,
						To:          payment.To,
						TokenValue:  payment.TokenValue,
						USDValue:    payment.USDValue,
						Status:      status,
						BlockHash:   payment.BlockHash,
						BlockNumber: payment.BlockNumber,
						Transaction: payment.Transaction,
						LogIndex:    payment.LogIndex,
						Timestamp:   payment.Timestamp,
					})
				}
			}
		}

		err = chore.paymentsDB.DeletePending(ctx)
		if err != nil {
			chore.log.Error("error removing pending payments from the DB", zap.Error(ChoreErr.Wrap(err)))
			return nil
		}

		err = chore.paymentsDB.InsertBatch(ctx, cachedPayments)
		if err != nil {
			chore.log.Error("error storing payments to db", zap.Error(ChoreErr.Wrap(err)))
			return nil
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
