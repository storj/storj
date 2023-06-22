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
	"storj.io/storj/satellite/payments/stripe"
)

var (
	// Error is the standard error class for automatic freeze errors.
	Error = errs.Class("account-freeze-chore")
	mon   = monkit.Package()
)

// Config contains configurable values for account freeze chore.
type Config struct {
	Enabled        bool          `help:"whether to run this chore." default:"false"`
	Interval       time.Duration `help:"How often to run this chore, which is how often unpaid invoices are checked." default:"24h"`
	GracePeriod    time.Duration `help:"How long to wait between a warning event and freezing an account." default:"360h"`
	PriceThreshold int64         `help:"The failed invoice amount (in cents) beyond which an account will not be frozen" default:"10000"`
}

// Chore is a chore that checks for unpaid invoices and potentially freezes corresponding accounts.
type Chore struct {
	log           *zap.Logger
	freezeService *console.AccountFreezeService
	analytics     *analytics.Service
	usersDB       console.Users
	payments      payments.Accounts
	accounts      stripe.DB
	config        Config
	nowFn         func() time.Time
	Loop          *sync2.Cycle
}

// NewChore is a constructor for Chore.
func NewChore(log *zap.Logger, accounts stripe.DB, payments payments.Accounts, usersDB console.Users, freezeService *console.AccountFreezeService, analytics *analytics.Service, config Config) *Chore {
	return &Chore{
		log:           log,
		freezeService: freezeService,
		analytics:     analytics,
		usersDB:       usersDB,
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
		bypassedMap := make(map[uuid.UUID]struct{})

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
				bypassedMap[userID] = struct{}{}
				debugLog("Ignoring invoice; amount exceeds threshold")
				chore.analytics.TrackLargeUnpaidInvoice(invoice.ID, userID, user.Email)
				continue
			}

			freeze, warning, err := chore.freezeService.GetAll(ctx, userID)
			if err != nil {
				errorLog("Could not get freeze status", err)
				continue
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
			zap.Int("total bypassed", len(bypassedMap)),
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
