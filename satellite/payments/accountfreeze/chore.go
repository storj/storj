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
	GracePeriod    time.Duration `help:"How long to wait between a warning event and freezing an account." default:"720h"`
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
			userMap[userID] = struct{}{}

			user, err := chore.usersDB.Get(ctx, userID)
			if err != nil {
				chore.log.Error("Could not get user",
					zap.String("invoiceID", invoice.ID),
					zap.String("customerID", invoice.CustomerID),
					zap.Any("userID", userID),
					zap.Error(Error.Wrap(err)),
				)
				continue
			}

			if invoice.Amount > chore.config.PriceThreshold {
				bypassedMap[userID] = struct{}{}
				chore.log.Debug("amount due over threshold",
					zap.String("invoiceID", invoice.ID),
					zap.String("customerID", invoice.CustomerID),
					zap.Any("userID", userID),
				)
				chore.analytics.TrackLargeUnpaidInvoice(invoice.ID, userID, user.Email)
				continue
			}

			freeze, warning, err := chore.freezeService.GetAll(ctx, userID)
			if err != nil {
				chore.log.Error("Could not check freeze status",
					zap.String("invoiceID", invoice.ID),
					zap.String("customerID", invoice.CustomerID),
					zap.Any("userID", userID),
					zap.Error(Error.Wrap(err)),
				)
				continue
			}
			if freeze != nil {
				chore.log.Debug("Ignoring invoice; account already frozen",
					zap.String("invoiceID", invoice.ID),
					zap.String("customerID", invoice.CustomerID),
					zap.Any("userID", userID),
				)
				continue
			}

			if warning == nil {
				err = chore.freezeService.WarnUser(ctx, userID)
				if err != nil {
					chore.log.Error("Could not add warning event",
						zap.String("invoiceID", invoice.ID),
						zap.String("customerID", invoice.CustomerID),
						zap.Any("userID", userID),
						zap.Error(Error.Wrap(err)),
					)
					continue
				}
				chore.log.Debug("user warned",
					zap.String("invoiceID", invoice.ID),
					zap.String("customerID", invoice.CustomerID),
					zap.Any("userID", userID),
				)
				warnedMap[userID] = struct{}{}
				continue
			}

			if chore.nowFn().Sub(warning.CreatedAt) > chore.config.GracePeriod {
				err = chore.freezeService.FreezeUser(ctx, userID)
				if err != nil {
					chore.log.Error("Could not freeze account",
						zap.String("invoiceID", invoice.ID),
						zap.String("customerID", invoice.CustomerID),
						zap.Any("userID", userID),
						zap.Error(Error.Wrap(err)),
					)
					continue
				}
				chore.log.Debug("user frozen",
					zap.String("invoiceID", invoice.ID),
					zap.String("customerID", invoice.CustomerID),
					zap.Any("userID", userID),
				)
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
