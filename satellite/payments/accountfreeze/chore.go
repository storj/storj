// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/stripe"
)

var (
	// Error is the standard error class for automatic freeze errors.
	Error = errs.Class("account-freeze-chore")
	mon   = monkit.Package()
)

// Config contains configurable values for account freeze chore.
type Config struct {
	Enabled          bool          `help:"whether to run this chore." default:"false"`
	Interval         time.Duration `help:"How often to run this chore, which is how often unpaid invoices are checked." default:"24h"`
	GracePeriod      time.Duration `help:"How long to wait between a warning event and freezing an account." default:"360h"`
	PriceThreshold   int64         `help:"The failed invoice amount (in cents) beyond which an account will not be frozen" default:"10000"`
	ExcludeStorjscan bool          `help:"whether to exclude storjscan-paying users from automatic warn/freeze" default:"true"`
}

// Chore is a chore that checks for unpaid invoices and potentially freezes corresponding accounts.
type Chore struct {
	log           *zap.Logger
	freezeService *console.AccountFreezeService
	analytics     *analytics.Service
	usersDB       console.Users
	walletsDB     storjscan.WalletsDB
	paymentsDB    storjscan.PaymentsDB
	payments      payments.Accounts
	accounts      stripe.DB
	config        Config
	nowFn         func() time.Time
	Loop          *sync2.Cycle
}

// NewChore is a constructor for Chore.
func NewChore(log *zap.Logger, accounts stripe.DB, payments payments.Accounts, usersDB console.Users, walletsDB storjscan.WalletsDB, paymentsDB storjscan.PaymentsDB, freezeService *console.AccountFreezeService, analytics *analytics.Service, config Config) *Chore {
	return &Chore{
		log:           log,
		freezeService: freezeService,
		analytics:     analytics,
		usersDB:       usersDB,
		walletsDB:     walletsDB,
		paymentsDB:    paymentsDB,
		accounts:      accounts,
		config:        config,
		payments:      payments,
		nowFn:         time.Now,
		Loop:          sync2.NewCycle(config.Interval),
	}
}

// Run runs the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {

		invoices, err := chore.payments.Invoices().ListFailed(ctx)
		if err != nil {
			chore.log.Error("Could not list invoices", zap.Error(Error.Wrap(err)))
			return nil
		}
		chore.log.Debug("failed invoices found", zap.Int("count", len(invoices)))

		userMap := make(map[uuid.UUID]struct{})
		frozenMap := make(map[uuid.UUID]struct{})
		warnedMap := make(map[uuid.UUID]struct{})
		bypassedLargeMap := make(map[uuid.UUID]struct{})
		bypassedTokenMap := make(map[uuid.UUID]struct{})

		checkInvPaid := func(invID string) (bool, error) {
			inv, err := chore.payments.Invoices().Get(ctx, invID)
			if err != nil {
				return false, err
			}
			return inv.Status == payments.InvoiceStatusPaid, nil
		}

		for _, invoice := range invoices {
			userID, err := chore.accounts.Customers().GetUserID(ctx, invoice.CustomerID)
			if err != nil {
				chore.log.Error("Could not get userID",
					zap.String("invoiceID", invoice.ID),
					zap.String("customerID", invoice.CustomerID),
					zap.Error(Error.Wrap(err)),
				)
				continue
			}

			debugLog := func(message string) {
				chore.log.Debug(message,
					zap.String("invoiceID", invoice.ID),
					zap.String("customerID", invoice.CustomerID),
					zap.Any("userID", userID),
				)
			}

			errorLog := func(message string, err error) {
				chore.log.Error(message,
					zap.String("invoiceID", invoice.ID),
					zap.String("customerID", invoice.CustomerID),
					zap.Any("userID", userID),
					zap.Error(Error.Wrap(err)),
				)
			}

			userMap[userID] = struct{}{}

			user, err := chore.usersDB.Get(ctx, userID)
			if err != nil {
				errorLog("Could not get user", err)
				continue
			}

			if invoice.Amount > chore.config.PriceThreshold {
				if _, ok := bypassedLargeMap[userID]; ok {
					continue
				}
				bypassedLargeMap[userID] = struct{}{}
				debugLog("Ignoring invoice; amount exceeds threshold")
				chore.analytics.TrackLargeUnpaidInvoice(invoice.ID, userID, user.Email)
				continue
			}

			if chore.config.ExcludeStorjscan {
				if _, ok := bypassedTokenMap[userID]; ok {
					continue
				}
				wallet, err := chore.walletsDB.GetWallet(ctx, user.ID)
				if err != nil && !errs.Is(err, billing.ErrNoWallet) {
					errorLog("Could not get wallets for user", err)
					continue
				}
				// if there is no error, the user has a wallet and we can check for transactions
				if err == nil {
					cachedPayments, err := chore.paymentsDB.ListWallet(ctx, wallet, 1, 0)
					if err != nil && !errs.Is(err, billing.ErrNoTransactions) {
						errorLog("Could not get payments for user", err)
						continue
					}
					if len(cachedPayments) > 0 {
						bypassedTokenMap[userID] = struct{}{}
						debugLog("Ignoring invoice; TX exists in storjscan")
						chore.analytics.TrackStorjscanUnpaidInvoice(invoice.ID, userID, user.Email)
						continue
					}
				}
			}

			freeze, warning, err := chore.freezeService.GetAll(ctx, userID)
			if err != nil {
				errorLog("Could not get freeze status", err)
				continue
			}

			// try to pay the invoice before freezing/warning.
			err = chore.payments.Invoices().AttemptPayOverdueInvoices(ctx, userID)
			if err == nil {
				debugLog("Ignoring invoice; Payment attempt successful")

				if warning != nil {
					err = chore.freezeService.UnWarnUser(ctx, userID)
					if err != nil {
						errorLog("Could not remove warning event", err)
					}
				}
				if freeze != nil {
					err = chore.freezeService.UnfreezeUser(ctx, userID)
					if err != nil {
						errorLog("Could not remove freeze event", err)
					}
				}

				continue
			} else {
				errorLog("Could not attempt payment", err)
			}

			if freeze != nil {
				debugLog("Ignoring invoice; account already frozen")
				continue
			}

			if warning == nil {
				// check if the invoice has been paid by the time the chore gets here.
				isPaid, err := checkInvPaid(invoice.ID)
				if err != nil {
					errorLog("Could not verify invoice status", err)
					continue
				}
				if isPaid {
					debugLog("Ignoring invoice; payment already made")
					continue
				}
				err = chore.freezeService.WarnUser(ctx, userID)
				if err != nil {
					errorLog("Could not add warning event", err)
					continue
				}
				debugLog("user warned")
				warnedMap[userID] = struct{}{}
				continue
			}

			if chore.nowFn().Sub(warning.CreatedAt) > chore.config.GracePeriod {
				// check if the invoice has been paid by the time the chore gets here.
				isPaid, err := checkInvPaid(invoice.ID)
				if err != nil {
					errorLog("Could not verify invoice status", err)
					continue
				}
				if isPaid {
					debugLog("Ignoring invoice; payment already made")
					continue
				}
				err = chore.freezeService.FreezeUser(ctx, userID)
				if err != nil {
					errorLog("Could not freeze account", err)
					continue
				}
				debugLog("user frozen")
				frozenMap[userID] = struct{}{}
			}
		}

		chore.log.Debug("chore executed",
			zap.Int("total invoices", len(invoices)),
			zap.Int("user total", len(userMap)),
			zap.Int("total warned", len(warnedMap)),
			zap.Int("total frozen", len(frozenMap)),
			zap.Int("total bypassed due to size of invoice", len(bypassedLargeMap)),
			zap.Int("total bypassed due to storjscan payments", len(bypassedTokenMap)),
		)

		return nil
	})
}

// TestSetNow sets nowFn on chore for testing.
func (chore *Chore) TestSetNow(f func() time.Time) {
	chore.nowFn = f
}

// TestSetFreezeService changes the freeze service for tests.
func (chore *Chore) TestSetFreezeService(service *console.AccountFreezeService) {
	chore.freezeService = service
}

// Close closes the chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
