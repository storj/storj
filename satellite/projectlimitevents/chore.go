// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package projectlimitevents

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
)

var (
	// Error is the standard error class for project limit events.
	Error = errs.Class("project limit events")
	mon   = monkit.Package()
)

// MailSender sends rendered emails. *mailservice.Service satisfies this interface.
type MailSender interface {
	SendRendered(ctx context.Context, to []post.Address, msg mailservice.Message) error
}

// Config holds configurable values for the project limit events chore.
type Config struct {
	Enabled         bool          `help:"enable project limit notification emails" default:"false"`
	EmailTimeBuffer time.Duration `help:"how long to wait before processing an event, to allow deduplication across API pods" default:"10m"`
	Interval        time.Duration `help:"how often to check the event queue" default:"5m"`
}

// Chore reads unprocessed project limit events and sends notification emails.
type Chore struct {
	log            *zap.Logger
	db             DB
	projects       console.Projects
	users          console.Users
	liveAccounting accounting.Cache
	mailService    MailSender
	config         Config
	nowFn          func() time.Time
	Loop           *sync2.Cycle
}

// NewChore creates a new Chore.
func NewChore(log *zap.Logger, db DB, projects console.Projects, users console.Users, liveAccounting accounting.Cache, mailService MailSender, config Config) *Chore {
	return &Chore{
		log:            log,
		db:             db,
		projects:       projects,
		users:          users,
		liveAccounting: liveAccounting,
		mailService:    mailService,
		config:         config,
		nowFn:          time.Now,
		Loop:           sync2.NewCycle(config.Interval),
	}
}

// Run runs the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !chore.config.Enabled {
		chore.log.Debug("chore is disabled, skipping")
		return nil
	}

	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		chore.log.Debug("chore processing")
		return chore.processWhileQueueHasItems(ctx)
	})
}

// Close closes the chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}

// processWhileQueueHasItems calls process in a loop until the queue is drained.
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

// process fetches the next batch (one project's events) and handles each event.
func (chore *Chore) process(ctx context.Context) (n int, err error) {
	defer mon.Task()(&ctx)(&err)

	batch, err := chore.db.GetNextBatch(ctx, chore.nowFn().Add(-chore.config.EmailTimeBuffer))
	if err != nil {
		return 0, err
	}
	if len(batch) == 0 {
		return 0, nil
	}

	projectID := batch[0].ProjectID

	project, err := chore.projects.Get(ctx, projectID)
	if err != nil {
		return 0, err
	}

	owner, err := chore.users.Get(ctx, project.OwnerID)
	if err != nil {
		return 0, err
	}

	notificationFlags := 0
	if project.NotificationFlags != nil {
		notificationFlags = *project.NotificationFlags
	}

	var processed []uuid.UUID
	var emailEventIDs []uuid.UUID
	var flagsToSet int
	var flagsToClear int

	for _, event := range batch {
		processed = append(processed, event.ID)

		if event.IsReset {
			// Clear the "email sent" bit if it was set.
			if notificationFlags&int(event.Event) != 0 {
				flagsToClear |= int(event.Event)
			}
		} else {
			// Skip if the feature-enable bit for this event category is not set.
			if notificationFlags&int(enableBitFor(event.Event)) == 0 {
				continue
			}
			// Skip if the "email sent" bit is already set (dedup across API pods).
			if notificationFlags&int(event.Event) != 0 {
				continue
			}
			flagsToSet |= int(event.Event)
			emailEventIDs = append(emailEventIDs, event.ID)
		}
	}

	// Send emails for newly triggered thresholds.
	// If both 80% and 100% events for the same category are in the batch, send only
	// the 100% email — the 80% one is stale. Both bits are still written to notification_flags.
	emailFlagsToSet := flagsToSet
	if emailFlagsToSet&int(accounting.StorageUsage100) != 0 {
		emailFlagsToSet &^= int(accounting.StorageUsage80)
	}
	if emailFlagsToSet&int(accounting.EgressUsage100) != 0 {
		emailFlagsToSet &^= int(accounting.EgressUsage80)
	}
	if emailFlagsToSet != 0 {
		if err = chore.sendEmails(ctx, owner.Email, project, emailFlagsToSet); err != nil {
			// Only update last_attempted on events that actually attempted email sending.
			return 0, errs.Combine(err, chore.db.UpdateLastAttempted(ctx, emailEventIDs, chore.nowFn()))
		}
	}

	// Update notification_flags in DB and Redis: set threshold bits, clear reset bits.
	if flagsToSet != 0 || flagsToClear != 0 {
		newFlags := (notificationFlags | flagsToSet) &^ flagsToClear
		project.NotificationFlags = &newFlags
		if err = chore.projects.Update(ctx, project); err != nil {
			return 0, err
		}
		if err = chore.liveAccounting.UpdateProjectNotificationFlags(ctx, projectID, newFlags); err != nil {
			chore.log.Error("failed to update redis notification flags", zap.String("project_public_id", project.PublicID.String()), zap.Error(err))
		}
	}

	if len(processed) > 0 {
		if err = chore.db.UpdateEmailSent(ctx, processed, chore.nowFn()); err != nil {
			return 0, err
		}
	}

	return len(batch), nil
}

// sendEmails sends the appropriate email for each newly set threshold bit.
func (chore *Chore) sendEmails(ctx context.Context, email string, project *console.Project, flagsToSet int) (err error) {
	defer mon.Task()(&ctx)(&err)

	to := []post.Address{{Address: email}}

	// Process thresholds from highest to lowest so the most urgent email is sent first.
	type threshold struct {
		bit accounting.ProjectUsageThreshold
		msg mailservice.Message
	}
	thresholds := []threshold{
		{accounting.StorageUsage100, &ProjectStorageUsage100Email{ProjectName: project.Name, Limit: derefSize(project.StorageLimit)}},
		{accounting.StorageUsage80, &ProjectStorageUsage80Email{ProjectName: project.Name, Limit: derefSize(project.StorageLimit)}},
		{accounting.EgressUsage100, &ProjectEgressUsage100Email{ProjectName: project.Name, Limit: derefSize(project.BandwidthLimit)}},
		{accounting.EgressUsage80, &ProjectEgressUsage80Email{ProjectName: project.Name, Limit: derefSize(project.BandwidthLimit)}},
	}

	for _, th := range thresholds {
		if flagsToSet&int(th.bit) == 0 {
			continue
		}
		if err = chore.mailService.SendRendered(ctx, to, th.msg); err != nil {
			return err
		}
	}
	return nil
}

// enableBitFor returns the feature-enable bit for the given threshold event type.
func enableBitFor(event accounting.ProjectUsageThreshold) accounting.ProjectUsageThreshold {
	switch event {
	case accounting.StorageUsage80, accounting.StorageUsage100:
		return accounting.StorageNotificationsEnabled
	case accounting.EgressUsage80, accounting.EgressUsage100:
		return accounting.EgressNotificationsEnabled
	default:
		return 0
	}
}

// derefSize dereferences a *memory.Size, returning 0 if nil.
func derefSize(s *memory.Size) memory.Size {
	if s == nil {
		return 0
	}
	return *s
}

// TestSetMailSender replaces the mail sender for testing.
func (chore *Chore) TestSetMailSender(s MailSender) {
	chore.mailService = s
}

// TestSetEnabled overrides the Enabled config flag for testing.
func (chore *Chore) TestSetEnabled(enabled bool) {
	chore.config.Enabled = enabled
}

// TestSetNow sets nowFn for testing.
func (chore *Chore) TestSetNow(f func() time.Time) {
	chore.nowFn = f
}

// TestRunOnce runs one full drain of the queue without going through the Loop cycle.
// For use in unit tests that don't start the chore goroutine.
func (chore *Chore) TestRunOnce(ctx context.Context) error {
	return chore.processWhileQueueHasItems(ctx)
}
