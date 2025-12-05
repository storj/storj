// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console/consoleweb/consoleapi/utils"
)

var (
	// Error is the standard error class for node events.
	Error = errs.Class("node events")
	mon   = monkit.Package()
)

// Config contains configurable values for node events chore.
type Config struct {
	Interval            time.Duration `help:"how long to wait before checking the node events DB again if there is nothing to work on" default:"5m"`
	SelectionWaitPeriod time.Duration `help:"how long the earliest instance of an event for a particular email should exist in the DB before it is selected" default:"5m"`
	Notifier            string        `help:"which notification provider to use" default:""`

	Customerio CustomerioConfig
}

// Notifier notifies node operators about node events.
type Notifier interface {
	// Notify notifies a node operator about an event that occurred on some of their nodes.
	Notify(ctx context.Context, satellite string, events []NodeEvent) (err error)
}

// Chore is a chore that reads events from node events and sends emails.
type Chore struct {
	log       *zap.Logger
	db        DB
	satellite string
	notifier  Notifier
	config    Config
	nowFn     func() time.Time
	Loop      *sync2.Cycle
}

// NewChore is a constructor for Chore.
func NewChore(log *zap.Logger, db DB, satellite string, notifier Notifier, config Config) *Chore {
	return &Chore{
		log:       log,
		db:        db,
		satellite: satellite,
		notifier:  notifier,
		config:    config,
		nowFn:     time.Now,
		Loop:      sync2.NewCycle(config.Interval),
	}
}

// Run runs the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, chore.processWhileQueueHasItems)
}

// processWhileQueueHasItems keeps calling process() until the DB is empty or something
// else goes wrong in fetching from the queue.
func (chore *Chore) processWhileQueueHasItems(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := chore.process(ctx)
		if err != nil {
			chore.log.Error("process", zap.Error(Error.Wrap(err)))
			return nil
		}
		if n == 0 {
			return nil
		}
	}
}

// process picks items from the DB, combines them into an email and sends it.
func (chore *Chore) process(ctx context.Context) (n int, err error) {
	defer mon.Task()(&ctx)(&err)

	batch, err := chore.db.GetNextBatch(ctx, chore.nowFn().Add(-chore.config.SelectionWaitPeriod))
	if err != nil {
		return 0, err
	}
	if len(batch) == 0 {
		return 0, nil
	}

	email := batch[0].Email

	var rowIDs []uuid.UUID
	for _, event := range batch {
		rowIDs = append(rowIDs, event.ID)
	}

	if utils.ValidateEmail(email) {
		if err = chore.notifier.Notify(ctx, chore.satellite, batch); err != nil {
			err = errs.Combine(err, chore.db.UpdateLastAttempted(ctx, rowIDs, chore.nowFn()))
			return 0, err
		}
	} else {
		chore.log.Error("invalid email", zap.String("email", email))
	}

	err = chore.db.UpdateEmailSent(ctx, rowIDs, chore.nowFn())
	return len(batch), err
}

// SetNotifier sets the notifier on chore for testing.
func (chore *Chore) SetNotifier(n Notifier) {
	chore.notifier = n
}

// SetNow sets nowFn on chore for testing.
func (chore *Chore) SetNow(f func() time.Time) {
	chore.nowFn = f
}

// Close closes the chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
