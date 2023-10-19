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
	PriceThreshold   int64         `help:"The failed invoice amount (in cents) beyond which an account will not be frozen" default:"100000"`
	ExcludeStorjscan bool          `help:"whether to exclude storjscan-paying users from automatic warn/freeze" default:"false"`
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

		chore.attemptBillingFreezeWarn(ctx)

		chore.attemptBillingUnfreezeUnwarn(ctx)

		return nil
	})
}

func (chore *Chore) attemptBillingFreezeWarn(ctx context.Context) {
	invoices, err := chore.payments.Invoices().ListFailed(ctx, nil)
	if err != nil {
		chore.log.Error("Could not list invoices", zap.Error(Error.Wrap(err)))
		return
	}
	chore.log.Info("failed invoices found", zap.Int("count", len(invoices)))

	userMap := make(map[uuid.UUID]struct{})
	billingFrozenMap := make(map[uuid.UUID]struct{})
	billingWarnedMap := make(map[uuid.UUID]struct{})
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

		infoLog := func(message string) {
			chore.log.Info(message,
				zap.String("process", "billing freeze/warn"),
				zap.String("invoiceID", invoice.ID),
				zap.String("customerID", invoice.CustomerID),
				zap.Any("userID", userID),
			)
		}

		errorLog := func(message string, err error) {
			chore.log.Error(message,
				zap.String("process", "billing freeze/warn"),
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

		if user.Status == console.Deleted {
			errorLog("Ignoring invoice; account already deleted", errs.New("user deleted, but has unpaid invoices"))
			continue
		}

		if invoice.Amount > chore.config.PriceThreshold {
			if _, ok := bypassedLargeMap[userID]; ok {
				continue
			}
			bypassedLargeMap[userID] = struct{}{}
			infoLog("Ignoring invoice; amount exceeds threshold")
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
					infoLog("Ignoring invoice; TX exists in storjscan")
					chore.analytics.TrackStorjscanUnpaidInvoice(invoice.ID, userID, user.Email)
					continue
				}
			}
		}

		freezes, err := chore.freezeService.GetAll(ctx, userID)
		if err != nil {
			errorLog("Could not get freeze status", err)
			continue
		}

		if freezes.ViolationFreeze != nil {
			infoLog("Ignoring invoice; account already frozen due to violation")
			chore.analytics.TrackViolationFrozenUnpaidInvoice(invoice.ID, userID, user.Email)
			continue
		}

		shouldEscalate := func(event *console.AccountFreezeEvent) bool {
			if event == nil || event.DaysTillEscalation == nil {
				return false
			}
			daysElapsed := int(chore.nowFn().Sub(event.CreatedAt).Hours() / 24)
			return daysElapsed > *event.DaysTillEscalation
		}

		if freezes.BillingFreeze != nil {
			if freezes.BillingFreeze.DaysTillEscalation == nil {
				continue
			}
			if shouldEscalate(freezes.BillingFreeze) {
				if user.Status == console.PendingDeletion {
					infoLog("Ignoring invoice; account already marked for deletion")
					continue
				}

				// check if the invoice has been paid by the time the chore gets here.
				isPaid, err := checkInvPaid(invoice.ID)
				if err != nil {
					errorLog("Could not verify invoice status", err)
					continue
				}
				if isPaid {
					infoLog("Ignoring invoice; payment already made")
					continue
				}

				err = chore.freezeService.EscalateBillingFreeze(ctx, userID, *freezes.BillingFreeze)
				if err != nil {
					errorLog("Could not mark account for deletion", err)
					continue
				}

				infoLog("account marked for deletion")
				continue
			}

			infoLog("Ignoring invoice; account already billing frozen")
			continue
		}

		if freezes.BillingWarning == nil {
			// check if the invoice has been paid by the time the chore gets here.
			isPaid, err := checkInvPaid(invoice.ID)
			if err != nil {
				errorLog("Could not verify invoice status", err)
				continue
			}
			if isPaid {
				infoLog("Ignoring invoice; payment already made")
				continue
			}
			err = chore.freezeService.BillingWarnUser(ctx, userID)
			if err != nil {
				errorLog("Could not add billing warning event", err)
				continue
			}
			infoLog("user billing warned")
			billingWarnedMap[userID] = struct{}{}
			continue
		}

		if freezes.BillingWarning.DaysTillEscalation == nil {
			continue
		}
		if shouldEscalate(freezes.BillingWarning) {
			// check if the invoice has been paid by the time the chore gets here.
			isPaid, err := checkInvPaid(invoice.ID)
			if err != nil {
				errorLog("Could not verify invoice status", err)
				continue
			}
			if isPaid {
				infoLog("Ignoring invoice; payment already made")
				continue
			}

			// try to pay the invoice before freezing.
			err = chore.payments.Invoices().AttemptPayOverdueInvoices(ctx, userID)
			if err == nil {
				infoLog("Ignoring invoice; Payment attempt successful")

				err = chore.freezeService.BillingUnWarnUser(ctx, userID)
				if err != nil {
					errorLog("Could not remove billing warning event", err)
				}

				continue
			} else {
				errorLog("Could not attempt payment", err)
			}

			err = chore.freezeService.BillingFreezeUser(ctx, userID)
			if err != nil {
				errorLog("Could not billing freeze account", err)
				continue
			}
			infoLog("user billing frozen")
			billingFrozenMap[userID] = struct{}{}
		}
	}

	chore.log.Info("billing freezing/warning executed",
		zap.Int("total invoices", len(invoices)),
		zap.Int("user total", len(userMap)),
		zap.Int("total billing warned", len(billingWarnedMap)),
		zap.Int("total billing frozen", len(billingFrozenMap)),
		zap.Int("total bypassed due to size of invoice", len(bypassedLargeMap)),
		zap.Int("total bypassed due to storjscan payments", len(bypassedTokenMap)),
	)
}

func (chore *Chore) attemptBillingUnfreezeUnwarn(ctx context.Context) {
	cursor := console.FreezeEventsCursor{
		Limit: 100,
	}
	hasNext := true
	usersCount := 0
	unwarnedCount := 0
	unfrozenCount := 0

	getEvents := func(c console.FreezeEventsCursor) (events *console.FreezeEventsPage, err error) {
		events, err = chore.freezeService.GetAllEvents(ctx, c)
		if err != nil {
			return nil, err
		}
		return events, err
	}

	for hasNext {
		events, err := getEvents(cursor)
		if err != nil {
			return
		}

		for _, event := range events.Events {
			errorLog := func(message string, err error) {
				chore.log.Error(message,
					zap.String("process", "billing unfreeze/unwarn"),
					zap.Any("userID", event.UserID),
					zap.String("eventType", event.Type.String()),
					zap.Error(Error.Wrap(err)),
				)
			}
			infoLog := func(message string) {
				chore.log.Info(message,
					zap.String("process", "billing unfreeze/unwarn"),
					zap.Any("userID", event.UserID),
					zap.String("eventType", event.Type.String()),
					zap.Error(Error.Wrap(err)),
				)
			}

			if event.Type == console.ViolationFreeze {
				infoLog("Skipping violation freeze event")
				continue
			}

			user, err := chore.usersDB.Get(ctx, event.UserID)
			if err != nil {
				errorLog("Could not get user", err)
				continue
			}

			if user.Status == console.Deleted || user.Status == console.PendingDeletion {
				infoLog("Skipping event; account already deleted or pending deletion")
				continue
			}

			usersCount++
			invoices, err := chore.payments.Invoices().ListFailed(ctx, &event.UserID)
			if err != nil {
				errorLog("Could not get failed invoices for user", err)
				continue
			}
			if len(invoices) > 0 {
				continue
			}

			if event.Type == console.BillingFreeze {
				err = chore.freezeService.BillingUnfreezeUser(ctx, event.UserID)
				if err != nil {
					errorLog("Could not billing unfreeze user", err)
				}
				unfrozenCount++
			} else if event.Type == console.BillingWarning {
				err = chore.freezeService.BillingUnWarnUser(ctx, event.UserID)
				if err != nil {
					errorLog("Could not billing unwarn user", err)
				}
				unwarnedCount++
			}
		}

		hasNext = events.Next
		if length := len(events.Events); length > 0 {
			cursor.StartingAfter = &events.Events[length-1].UserID
		}
	}

	chore.log.Info("billing unfreezing/unwarning executed",
		zap.Int("user total", usersCount),
		zap.Int("total unwarned", unwarnedCount),
		zap.Int("total unfrozen", unfrozenCount),
	)
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
