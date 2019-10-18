// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/stripe/stripe-go/client"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
)

var mon = monkit.Package()

// Error defines stripecoinpayments service error.
var Error = errs.Class("stripecoinpayments service error")

// Config stores needed information for payment service initialization.
type Config struct {
	StripeSecretKey        string
	CoinpaymentsPublicKey  string
	CoinpaymentsPrivateKey string
}

// Service is an implementation for payment service via Stripe and Coinpayments.
type Service struct {
	log              *zap.Logger
	customers        CustomersDB
	transactionsDB   TransactionsDB
	stripeClient     *client.API
	coinpayments     *coinpayments.Client
	transactionCycle sync2.Cycle
}

// NewService creates a Service instance.
func NewService(log *zap.Logger, config Config, customers CustomersDB, transactionsDB TransactionsDB) *Service {
	stripeClient := client.New(config.StripeSecretKey, nil)

	coinpaymentsClient := coinpayments.NewClient(
		coinpayments.Credentials{
			PublicKey:  config.CoinpaymentsPublicKey,
			PrivateKey: config.CoinpaymentsPrivateKey,
		},
	)

	return &Service{
		log:              log,
		customers:        customers,
		transactionsDB:   transactionsDB,
		stripeClient:     stripeClient,
		coinpayments:     coinpaymentsClient,
		transactionCycle: *sync2.NewCycle(time.Minute),
	}
}

// Accounts exposes all needed functionality to manage payment accounts.
func (service *Service) Accounts() payments.Accounts {
	return &accounts{service: service}
}

// Run runs payments clearing loop.
func (service *Service) Run(ctx context.Context) error {
	err := service.transactionCycle.Run(ctx,
		func(ctx context.Context) error {
			service.log.Info("running transactions update cycle")

			if err := service.updateTransactionsLoop(ctx); err != nil {
				service.log.Error("transaction update cycle failed", zap.Error(Error.Wrap(err)))
			}

			return nil
		},
	)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// Close closes payments clearing loop.
func (service *Service) Close() error {
	service.transactionCycle.Stop()
	return nil
}

// updateTransactionsLoop updates all pending transactions in a loop.
func (service *Service) updateTransactionsLoop(ctx context.Context) error {
	const (
		limit = 25
	)

	before := time.Now()

	txsPage, err := service.transactionsDB.ListPending(ctx, 0, limit, before)
	if err != nil {
		return err
	}

	if err := service.updateTransactions(ctx, txsPage.IDList()); err != nil {
		return err
	}

	for txsPage.Next {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		txsPage, err = service.transactionsDB.ListPending(ctx, txsPage.NextOffset, limit, before)
		if err != nil {
			return err
		}

		if err := service.updateTransactions(ctx, txsPage.IDList()); err != nil {
			return err
		}
	}

	return nil
}

// updateTransactions updates statuses and received amount for given transactions.
func (service *Service) updateTransactions(ctx context.Context, ids coinpayments.TransactionIDList) error {
	if len(ids) == 0 {
		service.log.Debug("no transactions found, skipping update")
		return nil
	}

	infos, err := service.coinpayments.Transactions().ListInfos(ctx, ids)
	if err != nil {
		return err
	}

	var updates []TransactionUpdate
	for id, info := range infos {
		updates = append(updates,
			TransactionUpdate{
				TransactionID: id,
				Status:        info.Status,
				Received:      info.Received,
			},
		)

		if info.Status == coinpayments.StatusReceived {
			// TODO: update balance for stripe cusotmers balance
		}
	}

	return service.transactionsDB.Update(ctx, updates)
}
