// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"fmt"
	"time"

	"storj.io/storj/internal/memory"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
)

var mon = monkit.Package()

// Error defines stripecoinpayments service error.
var Error = errs.Class("stripecoinpayments service error")

// Config stores needed information for payment service initialization.
type Config struct {
	StripeSecretKey              string        `help:"stripe API secret key" default:""`
	CoinpaymentsPublicKey        string        `help:"coinpayments API public key" default:""`
	CoinpaymentsPrivateKey       string        `help:"coinpayments API preivate key key" default:""`
	TransactionUpdateInterval    time.Duration `help:"amount of time we wait before running next transaction update loop" devDefault:"1m" releaseDefault:"30m"`
	AccountBalanceUpdateInterval time.Duration `help:"amount of time we wait before running next account balance update loop" devDefault:"3m" releaseDefault:"1h30m"`
}

// Service is an implementation for payment service via Stripe and Coinpayments.
type Service struct {
	log            *zap.Logger
	customers      CustomersDB
	transactionsDB TransactionsDB
	invoicesDB     InvoicesDB
	projectsDB     console.Projects
	accountingDB   accounting.ProjectAccounting
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

	const limit = 25
	before := time.Now()

	txsPage, err := service.transactionsDB.ListPending(ctx, 0, limit, before)
	if err != nil {
		return err
	}

	if err := service.updateTransactions(ctx, txsPage.IDList()); err != nil {
		return err
	}

	for txsPage.Next {
		if err = ctx.Err(); err != nil {
			return err
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
	var applies coinpayments.TransactionIDList

	for id, info := range infos {
		updates = append(updates,
			TransactionUpdate{
				TransactionID: id,
				Status:        info.Status,
				Received:      info.Received,
			},
		)

		// moment of transition to received state, which indicates
		// that customer funds were accepted, so we can apply this
		// amount to customer balance. So we create intent to update
		// customer balance in the future.
		if info.Status == coinpayments.StatusReceived {
			applies = append(applies, id)
		}
	}

	return service.transactionsDB.Update(ctx, updates, applies)
}

// applyAccountBalanceLoop fetches all unapplied transaction in a loop, applying transaction
// received amount to stripe customer balance.
func (service *Service) updateAccountBalanceLoop(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25
	before := time.Now()

	txsPage, err := service.transactionsDB.ListUnapplied(ctx, 0, limit, before)
	if err != nil {
		return err
	}

	for _, tx := range txsPage.Transactions {
		if err = ctx.Err(); err != nil {
			return err
		}

		if err = service.applyTransactionBalance(ctx, tx); err != nil {
			return err
		}
	}

	for txsPage.Next {
		if err = ctx.Err(); err != nil {
			return err
		}

		txsPage, err := service.transactionsDB.ListUnapplied(ctx, txsPage.NextOffset, limit, before)
		if err != nil {
			return err
		}

		for _, tx := range txsPage.Transactions {
			if err = ctx.Err(); err != nil {
				return err
			}

			if err = service.applyTransactionBalance(ctx, tx); err != nil {
				return err
			}
		}
	}

	return nil
}

// applyTransactionBalance applies transaction received amount to stripe customer balance.
func (service *Service) applyTransactionBalance(ctx context.Context, tx Transaction) (err error) {
	defer mon.Task()(&ctx)(&err)

	cusID, err := service.customers.GetCustomerID(ctx, tx.AccountID)
	if err != nil {
		return err
	}

	if err = service.transactionsDB.Consume(ctx, tx.ID); err != nil {
		return err
	}

	// TODO: add conversion logic
	amount, _ := tx.Received.Int64()

	params := &stripe.CustomerBalanceTransactionParams{
		Amount:      stripe.Int64(amount),
		Customer:    stripe.String(cusID),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String("storj token deposit"),
	}

	params.AddMetadata("txID", tx.ID.String())

	_, err = service.stripeClient.CustomerBalanceTransactions.New(params)
	return err
}

func (service *Service) PrepareProjectInvoicesItems(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25

	now := time.Now().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if before.After(now) {
		return Error.New("prepare is for past periods only")
	}

	projsPage, err := service.projectsDB.List2(ctx, 0, limit, before)
	if err != nil {
		return err
	}

	for _, project := range projsPage.Projects {
		// mb check row?

		// ms separate
		summ, err := service.accountingDB.ProjectSummary(ctx, project.ID, start, before)
		if err != nil {
			return err
		}


	}

	for projsPage.Next {
		if err = ctx.Err(); err != nil {
			return err
		}

		projsPage, err = service.projectsDB.List2(ctx, 0, limit, before)
		if err != nil {
			return err
		}


	}

	return nil
}

func (service *Service) createProjectInvoiceItems(ctx context.Context, projects []console.Projects) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

// CreateInvoices creates invoices for all accounts for specified billing period.
func (service *Service) PopulateInvoices(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25

	now := time.Now().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if before.After(now) {
		return Error.New("can not create invoices for future periods")
	}

	cusPage, err := service.customers.List(ctx, 0, limit, before)
	if err != nil {
		return err
	}

	for _, cus := range cusPage.Customers {
		if err = ctx.Err(); err != nil {
			return err
		}

		if err = service.customerPopulateInvoice(ctx, cus, start, before); err != nil {
			return err
		}
	}

	for cusPage.Next {
		if err = ctx.Err(); err != nil {
			return err
		}

		cusPage, err = service.customers.List(ctx, cusPage.NextOffset, limit, before)
		if err != nil {
			return err
		}

		for _, cus := range cusPage.Customers {
			if err = ctx.Err(); err != nil {
				return err
			}

			if err = service.customerPopulateInvoice(ctx, cus, start, before); err != nil {
				return err
			}
		}
	}

	return nil
}

// customerPopulateInvoice populates invoice with project usage for specific customer.
func (service *Service) customerPopulateInvoice(ctx context.Context, customer Customer, start, before time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25

	projsPage, err := service.projectsDB.List(ctx, customer.UserID, 0, limit, before)
	if err != nil {
		return err
	}

	for _, proj := range projsPage.Projects {
		if err = ctx.Err(); err != nil {
			return ctx.Err()
		}

		if err = service.projectPopulateInvoice(ctx, customer, proj, start, before); err != nil {
			return err
		}
	}

	for projsPage.Next {
		if err = ctx.Err(); err != nil {
			return err
		}

		projsPage, err = service.projectsDB.List(ctx, customer.UserID, 0, limit, before)
		if err != nil {
			return err
		}

		for _, proj := range projsPage.Projects {
			if err = ctx.Err(); err != nil {
				return ctx.Err()
			}

			if err = service.projectPopulateInvoice(ctx, customer, proj, start, before); err != nil {
				return err
			}
		}
	}

	return nil
}

func (service *Service) projectPopulateInvoice(ctx context.Context, cus Customer, proj console.Project, start, before time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err = service.invoicesDB.CheckProjectInvoicing(ctx, proj.ID, start, before); err != nil {
		return err
	}

	summ, err := service.accountingDB.ProjectSummary(ctx, proj.ID, start, before)
	if err != nil {
		return err
	}

	storageItem := &stripe.InvoiceItemParams{
		Amount:      stripe.Int64(0),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Customer:    stripe.String(cus.ID),
		Description: stripe.String(fmt.Sprintf("project %s, storage %f", proj.Name, memory.Size(summ.Storage).TB())),
		Period: &stripe.InvoiceItemPeriodParams{
			End:   stripe.Int64(before.Unix()),
			Start: stripe.Int64(start.Unix()),
		},
	}
	egressItem := &stripe.InvoiceItemParams{
		Amount:      stripe.Int64(0),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Customer:    stripe.String(cus.ID),
		Description: stripe.String(fmt.Sprintf("project %s, egress %f", proj.Name, memory.Size(summ.Egress).TB())),
		Period: &stripe.InvoiceItemPeriodParams{
			End:   stripe.Int64(before.Unix()),
			Start: stripe.Int64(start.Unix()),
		},
	}
	objectsItem := &stripe.InvoiceItemParams{
		Amount:      stripe.Int64(0),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Customer:    stripe.String(cus.ID),
		Description: stripe.String(fmt.Sprintf("project %s, objects %d", proj.Name, summ.Objects)),
		Period: &stripe.InvoiceItemPeriodParams{
			End:   stripe.Int64(before.Unix()),
			Start: stripe.Int64(start.Unix()),
		},
	}

	storageItem.AddMetadata("projectID", proj.ID.String())
	storageItem.AddMetadata("type", "storage")

	egressItem.AddMetadata("projectID", proj.ID.String())
	egressItem.AddMetadata("type", "egress")

	objectsItem.AddMetadata("projectID", proj.ID.String())
	objectsItem.AddMetadata("type", "objects")

	_, err = service.stripeClient.InvoiceItems.New(storageItem)
	if err != nil {
		return err
	}
	_, err = service.stripeClient.InvoiceItems.New(egressItem)
	if err != nil {
		return err
	}
	_, err = service.stripeClient.InvoiceItems.New(objectsItem)
	if err != nil {
		return err
	}

	return nil
}
