// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"fmt"
	"time"

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
	log              *zap.Logger
	customers        CustomersDB
	projectsDB       console.Projects
	accountingDB     accounting.ProjectAccounting
	transactionsDB   TransactionsDB
	projectRecordsDB ProjectRecordsDB
	stripeClient     *client.API
	coinPayments     *coinpayments.Client
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

// PrepareInvoiceProjectRecords iterates through all projects and creates invoice records if
// none exists.
func (service *Service) PrepareInvoiceProjectRecords(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25

	now := time.Now().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("prepare is for past periods only")
	}

	projsPage, err := service.projectsDB.List(ctx, 0, limit, end)
	if err != nil {
		return Error.Wrap(err)
	}

	if err = service.createProjectRecords(ctx, projsPage.Projects, start, end); err != nil {
		return Error.Wrap(err)
	}

	for projsPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		projsPage, err = service.projectsDB.List(ctx, 0, limit, end)
		if err != nil {
			return Error.Wrap(err)
		}

		if err = service.createProjectRecords(ctx, projsPage.Projects, start, end); err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// createProjectRecords creates invoice project record if none exists.
func (service *Service) createProjectRecords(ctx context.Context, projects []console.Project, start, end time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	var records []CreateProjectRecord
	for _, project := range projects {
		if err = ctx.Err(); err != nil {
			return err
		}

		if err = service.projectRecordsDB.Check(ctx, project.ID, start, end); err != nil {
			if err == ErrProjectRecordExists {
				continue
			}

			return err
		}

		summ, err := service.accountingDB.ProjectSummary(ctx, project.ID, start, end)
		if err != nil {
			return err
		}

		records = append(records,
			CreateProjectRecord{
				ProjectID: project.ID,
				Storage:   summ.Storage,
				Egress:    summ.Egress,
				Objects:   summ.Objects,
			},
		)
	}

	return service.projectRecordsDB.Create(ctx, records, start, end)
}

// InvoiceApplyProjectRecords iterates through unapplied invoice project records and creates invoice line items
// for stripe customer.
func (service *Service) InvoiceApplyProjectRecords(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25
	before := time.Now()

	recordsPage, err := service.projectRecordsDB.ListUnapplied(ctx, 0, limit, before)
	if err != nil {
		return err
	}

	if err = service.applyProjectRecords(ctx, recordsPage.Records); err != nil {
		return err
	}

	for recordsPage.Next {
		if err = ctx.Err(); err != nil {
			return err
		}

		recordsPage, err = service.projectRecordsDB.ListUnapplied(ctx, 0, limit, before)
		if err != nil {
			return err
		}

		if err = service.applyProjectRecords(ctx, recordsPage.Records); err != nil {
			return err
		}
	}

	return nil
}

// applyProjectRecords applies invoice intents as invoice line items to stripe customer.
func (service *Service) applyProjectRecords(ctx context.Context, records []ProjectRecord) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, record := range records {
		if err = ctx.Err(); err != nil {
			return err
		}

		proj, err := service.projectsDB.Get(ctx, record.ProjectID)
		if err != nil {
			return err
		}

		cusID, err := service.customers.GetCustomerID(ctx, proj.OwnerID)
		if err != nil {
			// TODO: add error to distinguish not set up account.
			// if account is not set up leave intent for future invoice
			continue
		}

		if err = service.createInvoiceItems(ctx, cusID, proj.Name, record); err != nil {
			return err
		}
	}

	return nil
}

// createInvoiceItems consumes invoice project record and creates invoice line items for stripe customer.
func (service *Service) createInvoiceItems(ctx context.Context, cusID, projName string, record ProjectRecord) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err = service.projectRecordsDB.Consume(ctx, record.ID); err != nil {
		return err
	}

	projectItem := &stripe.InvoiceItemParams{
		Amount:      stripe.Int64(0),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Customer:    stripe.String(cusID),
		Description: stripe.String(fmt.Sprintf("project %s", projName)),
		Period: &stripe.InvoiceItemPeriodParams{
			End:   stripe.Int64(record.PeriodEnd.Unix()),
			Start: stripe.Int64(record.PeriodStart.Unix()),
		},
	}

	projectItem.AddMetadata("projectID", record.ProjectID.String())
	projectItem.AddMetadata("type", "storage")

	_, err = service.stripeClient.InvoiceItems.New(projectItem)
	return
}

// CreateInvoices lists through all customers and creates invoices.
func (service *Service) CreateInvoices(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25
	before := time.Now()

	cusPage, err := service.customers.List(ctx, 0, limit, before)
	if err != nil {
		return Error.Wrap(err)
	}

	for _, cusID := range cusPage.Customers {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		if err = service.createInvoice(ctx, cusID); err != nil {
			return Error.Wrap(err)
		}
	}

	for cusPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		cusPage, err = service.customers.List(ctx, 0, limit, before)
		if err != nil {
			return Error.Wrap(err)
		}

		for _, cusID := range cusPage.Customers {
			if err = ctx.Err(); err != nil {
				return Error.Wrap(err)
			}

			if err = service.createInvoice(ctx, cusID); err != nil {
				return Error.Wrap(err)
			}
		}
	}

	return nil
}

// createInvoice creates invoice for stripe customer. Returns nil error if there are no
// pending invoice line items for customer.
func (service *Service) createInvoice(ctx context.Context, cusID string) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = service.stripeClient.Invoices.New(
		&stripe.InvoiceParams{
			Customer:    stripe.String(cusID),
			AutoAdvance: stripe.Bool(true),
		},
	)

	if err != nil {
		if stripeErr, ok := err.(*stripe.Error); ok {
			switch stripeErr.Code {
			case stripe.ErrorCodeInvoiceNoCustomerLineItems:
				return nil
			default:
				return err
			}
		}
	}

	return nil
}
