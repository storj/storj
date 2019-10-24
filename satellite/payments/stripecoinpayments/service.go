// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/stripe/stripe-go/client"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
)

var mon = monkit.Package()

// Error defines stripecoinpayments service error.
var Error = errs.Class("stripecoinpayments service error")

// Config stores needed information for payment service initialization.
type Config struct {
	StripeSecretKey           string        `help:"stripe API secret key" default:""`
	CoinpaymentsPublicKey     string        `help:"coinpayments API public key" default:""`
	CoinpaymentsPrivateKey    string        `help:"coinpayments API preivate key key" default:""`
	TransactionUpdateInterval time.Duration `help:"amount of time we wait before running next transaction update loop" devDefault:"1m" releaseDefault:"30m"`
}

// Service is an implementation for payment service via Stripe and Coinpayments.
type Service struct {
	log            *zap.Logger
	customers      CustomersDB
	transactionsDB TransactionsDB
	stripeClient   *client.API
	coinPayments   *coinpayments.Client
}

// NewService creates a Service instance.
func NewService(log *zap.Logger, config Config, customers CustomersDB, transactionsDB TransactionsDB) *Service {
	stripeClient := client.New(config.StripeSecretKey, nil)

	coinPaymentsClient := coinpayments.NewClient(
		coinpayments.Credentials{
			PublicKey:  config.CoinpaymentsPublicKey,
			PrivateKey: config.CoinpaymentsPrivateKey,
		},
	)

	return &Service{
		log:            log,
		customers:      customers,
		transactionsDB: transactionsDB,
		stripeClient:   stripeClient,
		coinPayments:   coinPaymentsClient,
	}
}

// Accounts exposes all needed functionality to manage payment accounts.
func (service *Service) Accounts() payments.Accounts {
	return &accounts{service: service}
}

// updateTransactionsLoop updates all pending transactions in a loop.
func (service *Service) updateTransactionsLoop(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

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
func (service *Service) updateTransactions(ctx context.Context, ids coinpayments.TransactionIDList) (err error) {
	defer mon.Task()(&ctx, ids)(&err)

	if len(ids) == 0 {
		service.log.Debug("no transactions found, skipping update")
		return nil
	}

	infos, err := service.coinPayments.Transactions().ListInfos(ctx, ids)
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

		// TODO: update stripe customer balance
	}

	return service.transactionsDB.Update(ctx, updates)
}
