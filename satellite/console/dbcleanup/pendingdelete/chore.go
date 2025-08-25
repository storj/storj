// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package pendingdelete

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/macaroon"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/payments"
)

var (
	// Error defines the pendingdelete chore errors class.
	Error           = errs.Class("pendingdelete")
	mon             = monkit.Package()
	userDataTask    = "user-pending-deletion"
	projectDataTask = "project-pending-deletion"
)

// Config contains configuration for pending deletion project cleanup.
type Config struct {
	Enabled           bool          `help:"whether (pending deletion) user/project data should be deleted or not" default:"false"`
	Interval          time.Duration `help:"how often to run this chore" default:"24h"`
	ListLimit         int           `help:"how many events to query in a batch" default:"100"`
	DeleteConcurrency int           `help:"how many delete workers to run at a time" default:"1"`

	Project         DeleteTypeConfig
	ViolationFreeze DeleteTypeConfig
	BillingFreeze   DeleteTypeConfig
	TrialFreeze     DeleteTypeConfig
}

// DeleteTypeConfig holds configuration for a specific type of pending deletion data to delete.
type DeleteTypeConfig struct {
	Enabled    bool          `help:"whether data of this type of pending deletion resource should be deleted or not" default:"false"`
	BufferTime time.Duration `help:"how long after the resource is marked for deletion should we wait before deleting data" default:"720h"`
}

// Chore completes deletion of data for projects
// that have been pending deletion for a while.
type Chore struct {
	log    *zap.Logger
	config Config

	accounts      payments.Accounts
	freezeService *console.AccountFreezeService

	bucketsDB buckets.DB
	metabase  *metabase.DB
	store     console.DB

	nowFn func() time.Time

	Loop *sync2.Cycle
}

// NewChore creates a new instance of this chore.
func NewChore(log *zap.Logger, config Config,
	accounts payments.Accounts, freezeService *console.AccountFreezeService,
	bucketsDB buckets.DB, consoleDB console.DB, metabase *metabase.DB,
) *Chore {
	return &Chore{
		log:    log,
		config: config,

		accounts:      accounts,
		freezeService: freezeService,

		metabase:  metabase,
		bucketsDB: bucketsDB,
		store:     consoleDB,
		nowFn:     time.Now,

		Loop: sync2.NewCycle(config.Interval),
	}
}

// Run starts this chore's loop.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !chore.config.Enabled {
		return nil
	}

	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		var group errgroup.Group

		if chore.config.Project.Enabled {
			group.Go(func() error {
				return chore.runDeleteProjects(ctx)
			})
		} else {
			chore.log.Info("skipping deleting pending deletion projects because it is disabled in config",
				zap.String("task", projectDataTask))
		}

		if len(chore.enabledFrozenDeleteTypes()) != 0 {
			group.Go(func() error {
				return chore.runDeleteUserData(ctx)
			})
		}

		return group.Wait()
	})
}

func (chore *Chore) runDeleteProjects(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	chore.log.Info("deleting pending deletion projects", zap.String("task", projectDataTask))

	mu := new(sync.Mutex)
	var errGrp errs.Group

	addErr := func(err error) {
		mu.Lock()
		errGrp.Add(err)
		mu.Unlock()
	}

	var skippedProjects, deletedProjects atomic.Int64
	hasNext := true
	for hasNext {
		idsPage, err := chore.store.Projects().ListPendingDeletionBefore(
			ctx, 0, // always on offset 0 because updating project status removes it from the list
			chore.config.ListLimit, chore.nowFn().Add(-chore.config.Project.BufferTime),
		)
		if err != nil {
			chore.log.Error("failed to get projects for deletion",
				zap.String("task", projectDataTask), zap.Error(err))
			return err
		}
		hasNext = idsPage.Next

		if !hasNext && len(idsPage.Ids) == 0 {
			break
		}

		limiter := sync2.NewLimiter(chore.config.DeleteConcurrency)

		for _, p := range idsPage.Ids {
			limiter.Go(ctx, func() {
				// confirm project still marked pending deletion
				project, err := chore.store.Projects().Get(ctx, p.ProjectID)
				if err != nil {
					chore.log.Error("failed to get project for deletion",
						zap.String("task", projectDataTask),
						zap.String("projectID", p.ProjectID.String()),
						zap.String("userID", p.OwnerID.String()),
						zap.Error(err),
					)
					addErr(err)
					return
				}

				if project.Status == nil || *project.Status != console.ProjectPendingDeletion {
					chore.log.Error("project not marked pending deletion, skipping",
						zap.String("task", projectDataTask),
						zap.String("projectID", p.ProjectID.String()),
						zap.String("userID", p.OwnerID.String()),
					)
					skippedProjects.Add(1)
					return
				}

				err = chore.deleteData(ctx, p.ProjectID, p.OwnerID, projectDataTask)
				if err != nil {
					addErr(err)
					return
				}

				err = chore.disableProject(ctx, p.ProjectID, p.OwnerID, projectDataTask)
				if err != nil {
					addErr(err)
					return
				}
				deletedProjects.Add(1)
			})
		}

		limiter.Wait()
	}

	chore.log.Info("finished deleting projects",
		zap.String("task", projectDataTask),
		zap.Int64("skipped_projects", skippedProjects.Load()),
		zap.Int64("deleted_projects", deletedProjects.Load()),
	)

	return Error.Wrap(errGrp.Err())
}

func (chore *Chore) enabledFrozenDeleteTypes() []console.EventTypeAndTime {
	var eventTypes []console.EventTypeAndTime
	if chore.config.ViolationFreeze.Enabled {
		eventTypes = append(eventTypes, console.EventTypeAndTime{
			EventType: console.ViolationFreeze,
			OlderThan: chore.nowFn().Add(-chore.config.ViolationFreeze.BufferTime),
		})
	}
	if chore.config.BillingFreeze.Enabled {
		eventTypes = append(eventTypes, console.EventTypeAndTime{
			EventType: console.BillingFreeze,
			OlderThan: chore.nowFn().Add(-chore.config.BillingFreeze.BufferTime),
		})
	}
	if chore.config.TrialFreeze.Enabled {
		eventTypes = append(eventTypes, console.EventTypeAndTime{
			EventType: console.TrialExpirationFreeze,
			OlderThan: chore.nowFn().Add(-chore.config.TrialFreeze.BufferTime),
		})
	}
	if len(eventTypes) == 0 {
		chore.log.Info("no freeze event types are enabled, skipping unpaid data deletion",
			zap.String("task", userDataTask),
		)
		return nil
	}

	return eventTypes
}

func (chore *Chore) runDeleteUserData(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	chore.log.Info("deleting pending deletion users and data", zap.String("task", userDataTask))

	mu := new(sync.Mutex)
	var errGrp errs.Group

	addErr := func(err error) {
		mu.Lock()
		errGrp.Add(err)
		mu.Unlock()
	}

	errorLog := func(msg string, err2 error, args ...zap.Field) {
		chore.log.Error(msg,
			zap.String("task", userDataTask),
			zap.Error(err2),
		)
	}

	var deletedUsers, skippedUsers, deletedProjects atomic.Int64
	eventTypes := chore.enabledFrozenDeleteTypes()
	hasMore := true
	for hasMore {
		events, err := chore.freezeService.GetEscalatedEventsBefore(ctx, console.GetEscalatedEventsBeforeParams{
			Limit:      chore.config.ListLimit,
			EventTypes: eventTypes,
		})
		if err != nil {
			errorLog("failed to get freeze events", err)
			return err
		}
		hasMore = len(events) >= chore.config.ListLimit

		if !hasMore && len(events) == 0 {
			break
		}

		limiter := sync2.NewLimiter(chore.config.DeleteConcurrency)

		for _, event := range events {
			limiter.Go(ctx, func() {
				// confirm user still marked pending deletion
				user, err := chore.store.Users().Get(ctx, event.UserID)
				if err != nil {
					errorLog("failed to get user for deletion", err, zap.String("userID", event.UserID.String()))
					addErr(err)
					return
				}

				if user.Status != console.PendingDeletion {
					chore.log.Error("user not marked pending deletion, skipping",
						zap.String("userID", event.UserID.String()),
						zap.String("task", userDataTask),
					)
					skippedUsers.Add(1)
					return
				}

				projects, err := chore.store.Projects().GetActiveByUserID(ctx, event.UserID)
				if err != nil {
					errorLog("failed to get projects for deletion", err, zap.String("userID", event.UserID.String()))
					addErr(err)
					return
				}

				for _, project := range projects {
					err := chore.deleteData(ctx, project.ID, event.UserID, userDataTask)
					if err != nil {
						addErr(err)
						return
					}

					err = chore.disableProject(ctx, project.ID, event.UserID, userDataTask)
					if err != nil {
						addErr(err)
						return
					}

					deletedProjects.Add(1)
				}

				// this is ideally part of chore.deactivateUser, but we cannot use a db object in
				// a transaction, which accounts.CreditCards().RemoveAll does internally.
				err = chore.accounts.CreditCards().RemoveAll(ctx, event.UserID)
				if err != nil {
					chore.log.Error("failed to remove user credit cards",
						zap.String("userID", event.UserID.String()),
						zap.Error(err),
					)
					addErr(err)
					return
				}

				err = chore.store.WithTx(ctx, func(ctx context.Context, tx console.DBTx) error {
					err = chore.deactivateUser(ctx, tx, event)
					if err != nil {
						return err
					}

					// remove the freeze event so this user is not retrieved by GetEscalatedEventsOlderThanToDelete
					err = tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, event.UserID, event.Type)
					if err != nil {
						chore.log.Error("failed to remove freeze event", zap.String("userID", event.UserID.String()), zap.Error(err))
						return err
					}

					deletedUsers.Add(1)

					return nil
				})
				if err != nil {
					addErr(err)
					return
				}
			})
		}

		limiter.Wait()
	}

	chore.log.Info("finished deleting pending deletion users and data",
		zap.String("task", userDataTask),
		zap.Int64("skipped_users", skippedUsers.Load()),
		zap.Int64("deleted_users", deletedUsers.Load()),
		zap.Int64("deleted_projects", deletedProjects.Load()),
	)

	return Error.Wrap(errGrp.Err())
}

func (chore *Chore) deleteData(ctx context.Context, projectID, ownerID uuid.UUID, task string) (err error) {
	mon.Task()(&ctx)(&err)

	// first list buckets and delete data contained within them.
	listOptions := buckets.ListOptions{
		Direction: buckets.DirectionForward,
	}

	allowedBuckets := macaroon.AllowedBuckets{
		All: true,
	}

	bucketList := buckets.List{More: true}
	for bucketList.More {
		bucketList, err = chore.bucketsDB.ListBuckets(ctx, projectID, listOptions, allowedBuckets)
		if err != nil {
			chore.log.Error("failed to list buckets",
				zap.String("userID", ownerID.String()),
				zap.String("projectID", projectID.String()),
				zap.Error(err),
			)
			return err
		}

		maxCommitDelay := 25 * time.Millisecond
		for _, bucket := range bucketList.Items {
			objectCount, err := chore.metabase.UncoordinatedDeleteAllBucketObjects(ctx, metabase.UncoordinatedDeleteAllBucketObjects{
				Bucket: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: metabase.BucketName(bucket.Name),
				},
				BatchSize:               100,
				StalenessTimestampBound: spanner.MaxStaleness(10 * time.Second),
				MaxCommitDelay:          &maxCommitDelay,
			})
			if err != nil {
				chore.log.Error(
					"failed to delete all bucket objects",
					zap.String("userID", ownerID.String()),
					zap.String("projectID", projectID.String()),
					zap.String("bucket", bucket.Name), zap.Error(err),
				)
				return err
			}
			chore.log.Info(
				"deleted data for bucket",
				zap.String("task", task),
				zap.Int64("objectCount", objectCount),
				zap.String("userID", ownerID.String()),
				zap.String("projectID", projectID.String()),
				zap.String("bucket", bucket.Name),
			)
		}
	}

	return nil
}

func (chore *Chore) disableProject(ctx context.Context, projectID, ownerID uuid.UUID, task string) (err error) {
	return chore.store.WithTx(ctx, func(ctx context.Context, tx console.DBTx) error {
		// delete project API keys.
		err = tx.APIKeys().DeleteAllByProjectID(ctx, projectID)
		if err != nil {
			chore.log.Error("failed to delete all API Keys for project",
				zap.String("task", task),
				zap.String("projectID", projectID.String()),
				zap.String("userID", ownerID.String()),
				zap.Error(err),
			)
			return err
		}

		// delete project domains.
		err = tx.Domains().DeleteAllByProjectID(ctx, projectID)
		if err != nil {
			chore.log.Error("failed to delete all domains for project",
				zap.String("task", task),
				zap.String("projectID", projectID.String()),
				zap.String("userID", ownerID.String()),
				zap.Error(err),
			)
		}

		// disable the project.
		err = tx.Projects().UpdateStatus(ctx, projectID, console.ProjectDisabled)
		if err != nil {
			chore.log.Error("failed to mark project as disabled",
				zap.String("task", task),
				zap.String("projectID", projectID.String()),
				zap.String("userID", ownerID.String()),
				zap.Error(err),
			)
			return err
		}

		chore.log.Info("marked project as disabled",
			zap.String("task", task),
			zap.String("projectID", projectID.String()),
			zap.String("userID", ownerID.String()),
		)
		return nil
	})
}

func (chore *Chore) deactivateUser(ctx context.Context, tx console.DBTx, event console.EventWithUser) (err error) {
	_, err = tx.WebappSessions().DeleteAllByUserID(ctx, event.UserID)
	if err != nil {
		chore.log.Error("failed to remove webapp sessions for user",
			zap.String("task", userDataTask),
			zap.String("userID", event.UserID.String()),
			zap.Error(err),
		)
		return err
	}

	deactivatedEmail := fmt.Sprintf("deactivated+%s@storj.io", event.UserID.String())
	status := console.Deleted

	err = tx.Users().Update(ctx, event.UserID, console.UpdateUserRequest{
		FullName:                    new(string),
		ShortName:                   new(*string),
		Email:                       &deactivatedEmail,
		Status:                      &status,
		ExternalID:                  new(*string),
		EmailChangeVerificationStep: new(int),
	})
	if err != nil {
		chore.log.Error("failed to update user status to Deleted",
			zap.String("task", userDataTask),
			zap.String("userID", event.UserID.String()),
			zap.Error(err),
		)
		return err
	}

	chore.log.Info(
		"user deactivated",
		zap.String("task", userDataTask),
		zap.String("userID", event.UserID.String()),
	)

	return nil
}

// Close stops chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}

// TestSetNowFn sets the function used to get the current time.
// This is only to be used in tests.
func (chore *Chore) TestSetNowFn(fn func() time.Time) {
	chore.nowFn = fn
}

// TestSetDeleteConcurrency sets the delete concurrency for the chore.
// This is only to be used in tests.
func (chore *Chore) TestSetDeleteConcurrency(concurrency int) {
	chore.config.DeleteConcurrency = concurrency
}
