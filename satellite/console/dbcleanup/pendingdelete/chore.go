// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package pendingdelete

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/macaroon"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/payments"
)

var (
	// Error defines the pendingdelete chore errors class.
	Error                     = errs.Class("pendingdelete")
	mon                       = monkit.Package()
	frozenDataTask            = "frozen-user-deletion"
	projectDataTask           = "project-pending-deletion"
	pendingDeleteUserDataTask = "user-pending-deletion"
)

// Config contains configuration for pending deletion project cleanup.
type Config struct {
	Enabled           bool          `help:"whether (pending deletion) user/project data should be deleted or not" default:"false"`
	Interval          time.Duration `help:"how often to run this chore" default:"24h"`
	ListLimit         int           `help:"how many events to query in a batch" default:"100"`
	DeleteConcurrency int           `help:"how many delete workers to run at a time" default:"1"`

	Project         DeleteTypeConfig
	User            DeleteTypeConfig
	ViolationFreeze DeleteTypeConfig
	BillingFreeze   DeleteTypeConfig
	TrialFreeze     DeleteTypeConfig
}

// namedDeleteType pairs a delete type config with the name its flags are
// registered under, so both can be reported together.
type namedDeleteType struct {
	name string
	cfg  DeleteTypeConfig
}

// deleteTypes returns every delete type config for iteration.
func (c Config) deleteTypes() []namedDeleteType {
	return []namedDeleteType{
		{"project", c.Project},
		{"user", c.User},
		{"violation-freeze", c.ViolationFreeze},
		{"billing-freeze", c.BillingFreeze},
		{"trial-freeze", c.TrialFreeze},
	}
}

// Validate checks that the enabled delete type configurations are safe to run with.
func (c Config) Validate() error {
	for _, deleteType := range c.deleteTypes() {
		if !deleteType.cfg.Enabled {
			continue
		}
		// a buffer time longer than the lock buffer time would delete the data of
		// buckets with object lock enabled before that of buckets without it.
		if deleteType.cfg.BufferTime > deleteType.cfg.LockBufferTime {
			return Error.New("%s: buffer-time (%s) must not be greater than lock-buffer-time (%s)",
				deleteType.name, deleteType.cfg.BufferTime, deleteType.cfg.LockBufferTime)
		}
	}
	return nil
}

// warnUnbufferedDeletes logs the enabled delete types whose data in buckets
// without object lock is deleted as soon as the resource is picked up, leaving no
// window to catch a resource that was marked for deletion by mistake.
func warnUnbufferedDeletes(log *zap.Logger, config Config) {
	for _, deleteType := range config.deleteTypes() {
		if deleteType.cfg.Enabled && deleteType.cfg.BufferTime == 0 {
			log.Warn("buffer time is zero, data in buckets without object lock is deleted as soon as the resource is picked up",
				zap.String("delete_type", deleteType.name),
			)
		}
	}
}

// DeleteTypeConfig holds configuration for a specific type of pending deletion data to delete.
type DeleteTypeConfig struct {
	Enabled        bool          `help:"whether data of this type of pending deletion resource should be deleted or not" default:"false"`
	BufferTime     time.Duration `help:"how long after the resource is marked for deletion to wait before deleting data in buckets without object lock enabled" default:"0h"`
	LockBufferTime time.Duration `help:"how long after the resource is marked for deletion to wait before deleting data in buckets with object lock enabled" default:"720h"`
}

// listBufferTime is the buffer time used when listing resources for deletion.
// It is the earlier of the two thresholds so that a resource becomes visible as
// soon as it has any data eligible for deletion.
func (c DeleteTypeConfig) listBufferTime() time.Duration {
	if c.BufferTime < c.LockBufferTime {
		return c.BufferTime
	}
	return c.LockBufferTime
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

	remainderChargeRecorder *accounting.RemainderChargeRecorder

	nowFn func() time.Time

	Loop *sync2.Cycle
}

// NewChore creates a new instance of this chore.
// remainderChargeRecorder can be nil to disable remainder charge tracking.
func NewChore(log *zap.Logger, accounts payments.Accounts,
	freezeService *console.AccountFreezeService,
	bucketsDB buckets.DB, consoleDB console.DB, metabase *metabase.DB,
	remainderChargeRecorder *accounting.RemainderChargeRecorder,
	config Config,
) *Chore {
	warnUnbufferedDeletes(log, config)

	return &Chore{
		log:    log,
		config: config,

		accounts:      accounts,
		freezeService: freezeService,

		metabase:  metabase,
		bucketsDB: bucketsDB,
		store:     consoleDB,

		remainderChargeRecorder: remainderChargeRecorder,

		nowFn: time.Now,

		Loop: sync2.NewCycle(config.Interval),
	}
}

// Run starts this chore's loop.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !chore.config.Enabled {
		return nil
	}

	if err := chore.config.Validate(); err != nil {
		return err
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
				return chore.runDeleteFrozenUsers(ctx)
			})
		}

		if chore.config.User.Enabled {
			group.Go(func() error {
				return chore.runDeletePendingDeletionUsers(ctx)
			})
		} else {
			chore.log.Info("skipping deleting pending deletion users because it is disabled in config",
				zap.String("task", pendingDeleteUserDataTask))
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

	var skippedProjects, deletedProjects, retainedProjects atomic.Int64
	cfg := chore.config.Project
	var offset int64
	hasNext := true
	for hasNext {
		idsPage, err := chore.store.Projects().ListPendingDeletionBefore(
			ctx, offset,
			chore.config.ListLimit, chore.nowFn().Add(-cfg.listBufferTime()),
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

		// stayed counts resources still pending deletion after this batch (retained
		// for object lock, or failed). The next page starts after them, since
		// finalized resources leave the list but retained ones do not.
		var stayed atomic.Int64
		limiter := sync2.NewLimiter(chore.config.DeleteConcurrency)

		for _, p := range idsPage.Ids {
			limiter.Go(ctx, func() {
				// confirm project still marked pending deletion
				project, err := chore.store.Projects().Get(ctx, p.ProjectID)
				if err != nil {
					chore.log.Error("failed to get project for deletion",
						zap.String("task", projectDataTask),
						zap.String("public_project_id", p.ProjectPublicID.String()),
						zap.String("user_id", p.OwnerID.String()),
						zap.Error(err),
					)
					addErr(err)
					stayed.Add(1)
					return
				}

				if project.Status == nil || *project.Status != console.ProjectPendingDeletion {
					chore.log.Info("project not marked pending deletion, skipping",
						zap.String("task", projectDataTask),
						zap.String("public_project_id", p.ProjectPublicID.String()),
						zap.String("user_id", p.OwnerID.String()),
					)
					skippedProjects.Add(1)
					return
				}

				deleteUnlocked, deleteLocked := chore.bucketDeletability(project.StatusUpdatedAt, cfg)
				retainedBuckets, err := chore.deleteData(ctx, p.ProjectID, p.ProjectPublicID, p.OwnerID, projectDataTask, true, deleteUnlocked, deleteLocked)
				if err != nil {
					addErr(err)
					stayed.Add(1)
					return
				}

				// if the project still has buckets whose deletion threshold has not
				// elapsed (object lock within its retention window), retain their data
				// and keep the project pending deletion.
				if retainedBuckets > 0 {
					chore.log.Info("project has buckets within their retention window, retaining data and skipping project deletion",
						zap.String("task", projectDataTask),
						zap.String("public_project_id", p.ProjectPublicID.String()),
						zap.String("user_id", p.OwnerID.String()),
						zap.Int("retained_buckets", retainedBuckets),
					)
					retainedProjects.Add(1)
					stayed.Add(1)
					return
				}

				err = chore.disableProject(ctx, p.ProjectID, p.ProjectPublicID, p.OwnerID, projectDataTask)
				if err != nil {
					addErr(err)
					stayed.Add(1)
					return
				}
				deletedProjects.Add(1)
			})
		}

		limiter.Wait()
		offset += stayed.Load()
	}

	chore.log.Info("finished deleting projects",
		zap.String("task", projectDataTask),
		zap.Int64("skipped_projects", skippedProjects.Load()),
		zap.Int64("retained_projects", retainedProjects.Load()),
		zap.Int64("deleted_projects", deletedProjects.Load()),
	)

	return Error.Wrap(errGrp.Err())
}

func (chore *Chore) runDeletePendingDeletionUsers(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	chore.log.Info("deleting pending deletion users", zap.String("task", pendingDeleteUserDataTask))

	mu := new(sync.Mutex)
	var errGrp errs.Group

	addErr := func(err error) {
		mu.Lock()
		errGrp.Add(err)
		mu.Unlock()
	}

	errorLog := func(msg string, err2 error, args ...zap.Field) {
		chore.log.Error(msg, append([]zap.Field{
			zap.String("task", pendingDeleteUserDataTask),
			zap.Error(err2),
		}, args...)...)
	}

	var skippedUsers, deletedUsers, retainedUsers, deletedProjects atomic.Int64
	cfg := chore.config.User
	var offset int64
	hasNext := true
	for hasNext {
		idsPage, err := chore.store.Users().ListPendingDeletionBefore(
			ctx, offset,
			chore.config.ListLimit, chore.nowFn().Add(-cfg.listBufferTime()),
		)
		if err != nil {
			chore.log.Error("failed to get users for deletion",
				zap.String("task", pendingDeleteUserDataTask), zap.Error(err))
			return err
		}
		hasNext = idsPage.HasNext

		if !hasNext && len(idsPage.IDs) == 0 {
			break
		}

		var stayed atomic.Int64
		limiter := sync2.NewLimiter(chore.config.DeleteConcurrency)

		for _, userID := range idsPage.IDs {
			limiter.Go(ctx, func() {
				// confirm user still marked pending deletion
				user, err := chore.store.Users().Get(ctx, userID)
				if err != nil {
					chore.log.Error("failed to get user for deletion",
						zap.String("task", pendingDeleteUserDataTask),
						zap.String("user_id", userID.String()),
						zap.Error(err),
					)
					addErr(err)
					stayed.Add(1)
					return
				}

				if user.Status != console.PendingDeletion {
					chore.log.Info("user not marked pending deletion, skipping",
						zap.String("task", pendingDeleteUserDataTask),
						zap.String("user_id", userID.String()),
					)
					skippedUsers.Add(1)
					return
				}

				projects, err := chore.store.Projects().GetOwnActive(ctx, userID)
				if err != nil {
					errorLog("failed to get projects for deletion", err, zap.String("user_id", userID.String()))
					addErr(err)
					stayed.Add(1)
					return
				}

				deleteUnlocked, deleteLocked := chore.bucketDeletability(user.StatusUpdatedAt, cfg)
				var retainedBuckets int
				for _, project := range projects {
					r, err := chore.deleteData(ctx, project.ID, project.PublicID, userID, pendingDeleteUserDataTask, false, deleteUnlocked, deleteLocked)
					if err != nil {
						addErr(err)
						stayed.Add(1)
						return
					}

					// retain projects whose buckets are still within their window.
					if r > 0 {
						retainedBuckets += r
						continue
					}

					err = chore.disableProject(ctx, project.ID, project.PublicID, userID, pendingDeleteUserDataTask)
					if err != nil {
						addErr(err)
						stayed.Add(1)
						return
					}

					deletedProjects.Add(1)
				}

				// don't deactivate the user while any of their projects still hold
				// retained (object lock) data.
				if retainedBuckets > 0 {
					chore.log.Info("user has buckets within their retention window, retaining data and skipping user deletion",
						zap.String("task", pendingDeleteUserDataTask),
						zap.String("user_id", userID.String()),
						zap.Int("retained_buckets", retainedBuckets),
					)
					retainedUsers.Add(1)
					stayed.Add(1)
					return
				}

				err = chore.deactivateUser(ctx, userID, nil, pendingDeleteUserDataTask)
				if err != nil {
					addErr(err)
					stayed.Add(1)
					return
				}
				deletedUsers.Add(1)
			})
		}

		limiter.Wait()
		offset += stayed.Load()
	}

	chore.log.Info("finished deleting users",
		zap.String("task", pendingDeleteUserDataTask),
		zap.Int64("skipped_users", skippedUsers.Load()),
		zap.Int64("retained_users", retainedUsers.Load()),
		zap.Int64("deleted_users", deletedUsers.Load()),
		zap.Int64("deleted_projects", deletedProjects.Load()),
	)

	return Error.Wrap(errGrp.Err())
}

func (chore *Chore) enabledFrozenDeleteTypes() []console.EventTypeAndTime {
	var eventTypes []console.EventTypeAndTime
	if chore.config.ViolationFreeze.Enabled {
		eventTypes = append(eventTypes, console.EventTypeAndTime{
			EventType: console.ViolationFreeze,
			OlderThan: chore.nowFn().Add(-chore.config.ViolationFreeze.listBufferTime()),
		})
	}
	if chore.config.BillingFreeze.Enabled {
		eventTypes = append(eventTypes, console.EventTypeAndTime{
			EventType: console.BillingFreeze,
			OlderThan: chore.nowFn().Add(-chore.config.BillingFreeze.listBufferTime()),
		})
	}
	if chore.config.TrialFreeze.Enabled {
		eventTypes = append(eventTypes, console.EventTypeAndTime{
			EventType: console.TrialExpirationFreeze,
			OlderThan: chore.nowFn().Add(-chore.config.TrialFreeze.listBufferTime()),
		})
	}
	if len(eventTypes) == 0 {
		chore.log.Info("no freeze event types are enabled, skipping unpaid data deletion",
			zap.String("task", frozenDataTask),
		)
		return nil
	}

	return eventTypes
}

// freezeDeleteConfig returns the delete config matching a freeze event type.
func (chore *Chore) freezeDeleteConfig(eventType console.AccountFreezeEventType) (DeleteTypeConfig, error) {
	switch eventType {
	case console.ViolationFreeze:
		return chore.config.ViolationFreeze, nil
	case console.BillingFreeze:
		return chore.config.BillingFreeze, nil
	case console.TrialExpirationFreeze:
		return chore.config.TrialFreeze, nil
	default:
		return DeleteTypeConfig{}, Error.New("no delete config for freeze event type %d", eventType)
	}
}

func (chore *Chore) runDeleteFrozenUsers(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	chore.log.Info("deleting pending deletion users and data", zap.String("task", frozenDataTask))

	mu := new(sync.Mutex)
	var errGrp errs.Group

	addErr := func(err error) {
		mu.Lock()
		errGrp.Add(err)
		mu.Unlock()
	}

	errorLog := func(msg string, err2 error, args ...zap.Field) {
		chore.log.Error(msg, append([]zap.Field{
			zap.String("task", frozenDataTask),
			zap.Error(err2),
		}, args...)...)
	}

	var deletedUsers, skippedUsers, retainedUsers, deletedProjects atomic.Int64
	eventTypes := chore.enabledFrozenDeleteTypes()
	var offset int
	hasMore := true
	for hasMore {
		events, err := chore.freezeService.GetEscalatedEventsBefore(ctx, console.GetEscalatedEventsBeforeParams{
			Limit:      chore.config.ListLimit,
			Offset:     offset,
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

		var stayed atomic.Int64
		limiter := sync2.NewLimiter(chore.config.DeleteConcurrency)

		for _, event := range events {
			limiter.Go(ctx, func() {
				// confirm user still marked pending deletion
				user, err := chore.store.Users().Get(ctx, event.UserID)
				if err != nil {
					errorLog("failed to get user for deletion", err, zap.String("user_id", event.UserID.String()))
					addErr(err)
					stayed.Add(1)
					return
				}

				if user.Status != console.PendingDeletion {
					chore.log.Info("user not marked pending deletion, skipping",
						zap.String("user_id", event.UserID.String()),
						zap.String("task", frozenDataTask),
					)
					skippedUsers.Add(1)
					return
				}

				projects, err := chore.store.Projects().GetOwnActive(ctx, event.UserID)
				if err != nil {
					errorLog("failed to get projects for deletion", err, zap.String("user_id", event.UserID.String()))
					addErr(err)
					stayed.Add(1)
					return
				}

				cfg, err := chore.freezeDeleteConfig(event.Type)
				if err != nil {
					errorLog("failed to get delete config for freeze event", err,
						zap.String("user_id", event.UserID.String()))
					addErr(err)
					stayed.Add(1)
					return
				}

				deleteUnlocked, deleteLocked := chore.bucketDeletability(user.StatusUpdatedAt, cfg)
				var retainedBuckets int
				for _, project := range projects {
					r, err := chore.deleteData(ctx, project.ID, project.PublicID, event.UserID, frozenDataTask, false, deleteUnlocked, deleteLocked)
					if err != nil {
						addErr(err)
						stayed.Add(1)
						return
					}

					// retain projects whose buckets are still within their window.
					if r > 0 {
						retainedBuckets += r
						continue
					}

					err = chore.disableProject(ctx, project.ID, project.PublicID, event.UserID, frozenDataTask)
					if err != nil {
						addErr(err)
						stayed.Add(1)
						return
					}

					deletedProjects.Add(1)
				}

				// don't deactivate the user while any of their projects still hold
				// retained (object lock) data.
				if retainedBuckets > 0 {
					chore.log.Info("user has buckets within their retention window, retaining data and skipping user deletion",
						zap.String("task", frozenDataTask),
						zap.String("user_id", event.UserID.String()),
						zap.Int("retained_buckets", retainedBuckets),
					)
					retainedUsers.Add(1)
					stayed.Add(1)
					return
				}

				err = chore.deactivateUser(ctx, event.UserID, &event.Type, frozenDataTask)
				if err != nil {
					addErr(err)
					stayed.Add(1)
					return
				}
				deletedUsers.Add(1)
			})
		}

		limiter.Wait()
		offset += int(stayed.Load())
	}

	chore.log.Info("finished deleting pending deletion users and data",
		zap.String("task", frozenDataTask),
		zap.Int64("skipped_users", skippedUsers.Load()),
		zap.Int64("retained_users", retainedUsers.Load()),
		zap.Int64("deleted_users", deletedUsers.Load()),
		zap.Int64("deleted_projects", deletedProjects.Load()),
	)

	return Error.Wrap(errGrp.Err())
}

// deleteData deletes the objects contained in the project's buckets whose
// deletion threshold has elapsed: buckets without object lock are deleted when
// deleteUnlocked is true, buckets with object lock enabled when deleteLocked is
// true. Buckets whose threshold has not yet elapsed are retained; the number of
// retained buckets is returned so the caller can decide whether the
// project/account can be fully deleted yet.
func (chore *Chore) deleteData(ctx context.Context, projectID, projectPublicID, ownerID uuid.UUID, task string, trackRemainder, deleteUnlocked, deleteLocked bool) (retainedBuckets int, err error) {
	defer mon.Task()(&ctx)(&err)

	// first list buckets and delete data contained within them.
	listOptions := buckets.ListOptions{
		Direction: buckets.DirectionForward,
	}

	allowedBuckets := macaroon.AllowedBuckets{
		All: true,
	}

	shouldTrackRemainder := trackRemainder && chore.remainderChargeRecorder != nil

	bucketList := buckets.List{More: true}
	for bucketList.More {
		bucketList, err = chore.bucketsDB.ListBuckets(ctx, projectID, listOptions, allowedBuckets)
		if err != nil {
			chore.log.Error("failed to list buckets",
				zap.String("user_id", ownerID.String()),
				zap.String("public_project_id", projectPublicID.String()),
				zap.Error(err),
			)
			return retainedBuckets, err
		}

		maxCommitDelay := 25 * time.Millisecond
		for _, bucket := range bucketList.Items {
			if bucket.ObjectLock.Enabled {
				// retain object lock data until its (longer) threshold elapses.
				if !deleteLocked {
					retainedBuckets++
					chore.log.Info("bucket has object lock enabled and is within the retention window, retaining data",
						zap.String("task", task),
						zap.String("user_id", ownerID.String()),
						zap.String("public_project_id", projectPublicID.String()),
						zap.String("bucket", bucket.Name),
					)
					continue
				}
				// threshold elapsed: object lock is intentionally overridden for
				// terminated (pending deletion) accounts.
				chore.log.Info("object lock retention window elapsed, deleting bucket data",
					zap.String("task", task),
					zap.String("user_id", ownerID.String()),
					zap.String("public_project_id", projectPublicID.String()),
					zap.String("bucket", bucket.Name),
				)
			} else if !deleteUnlocked {
				retainedBuckets++
				continue
			}

			bucketName := bucket.Name
			bucketPlacement := bucket.Placement

			var onObjectsDeleted func([]metabase.DeleteObjectsInfo)
			if shouldTrackRemainder {
				onObjectsDeleted = func(batchInfo []metabase.DeleteObjectsInfo) {
					chore.remainderChargeRecorder.Record(ctx, accounting.RecordRemainderChargesParams{
						ProjectID:       projectID,
						ProjectPublicID: projectPublicID,
						BucketName:      bucketName,
						Placement:       bucketPlacement,
						ObjectsFunc: func() []metabase.DeleteObjectsInfo {
							return batchInfo
						},
						DeletedAt: chore.nowFn(),
					})
				}
			}

			objectCount, err := chore.metabase.UncoordinatedDeleteAllBucketObjects(ctx, metabase.UncoordinatedDeleteAllBucketObjects{
				Bucket: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: metabase.BucketName(bucket.Name),
				},
				BatchSize:        100,
				MaxCommitDelay:   &maxCommitDelay,
				OnObjectsDeleted: onObjectsDeleted,
			})
			if err != nil {
				chore.log.Error(
					"failed to delete all bucket objects",
					zap.String("user_id", ownerID.String()),
					zap.String("public_project_id", projectPublicID.String()),
					zap.String("bucket", bucket.Name), zap.Error(err),
				)
				return retainedBuckets, err
			}

			chore.log.Info(
				"deleted data for bucket",
				zap.String("task", task),
				zap.Int64("object_count", objectCount),
				zap.String("user_id", ownerID.String()),
				zap.String("public_project_id", projectPublicID.String()),
				zap.String("bucket", bucket.Name),
			)
		}

		// advance the cursor so the next iteration lists the following page
		// rather than re-listing (and re-counting retained buckets from) this one.
		listOptions = listOptions.NextPage(bucketList)
	}

	return retainedBuckets, nil
}

// bucketDeletability reports, for a resource marked for deletion at markedAt,
// whether its buckets without object lock and its buckets with object lock may
// have their data deleted yet, according to the given config's thresholds.
// A nil markedAt (legacy rows without a status timestamp) is treated as long past due.
func (chore *Chore) bucketDeletability(markedAt *time.Time, cfg DeleteTypeConfig) (deleteUnlocked, deleteLocked bool) {
	if markedAt == nil {
		return true, true
	}
	now := chore.nowFn()
	deleteUnlocked = !markedAt.After(now.Add(-cfg.BufferTime))
	deleteLocked = !markedAt.After(now.Add(-cfg.LockBufferTime))
	return deleteUnlocked, deleteLocked
}

func (chore *Chore) disableProject(ctx context.Context, projectID, projectPublicID, ownerID uuid.UUID, task string) (err error) {
	return chore.store.WithTx(ctx, func(ctx context.Context, tx console.DBTx) error {
		// delete project API keys.
		err = tx.APIKeys().DeleteAllByProjectID(ctx, projectID)
		if err != nil {
			chore.log.Error("failed to delete all API Keys for project",
				zap.String("task", task),
				zap.String("public_project_id", projectPublicID.String()),
				zap.String("user_id", ownerID.String()),
				zap.Error(err),
			)
			return err
		}

		// remove project entitlements.
		err = tx.Entitlements().DeleteByScope(ctx, entitlements.ConvertPublicIDToProjectScope(projectPublicID))
		if err != nil {
			chore.log.Error("failed to delete project entitlements",
				zap.String("task", task),
				zap.String("public_project_id", projectPublicID.String()),
				zap.String("user_id", ownerID.String()),
				zap.Error(err),
			)
		}

		// delete project domains.
		err = tx.Domains().DeleteAllByProjectID(ctx, projectID)
		if err != nil {
			chore.log.Error("failed to delete all domains for project",
				zap.String("task", task),
				zap.String("public_project_id", projectPublicID.String()),
				zap.String("user_id", ownerID.String()),
				zap.Error(err),
			)
		}

		// disable the project.
		err = tx.Projects().UpdateStatus(ctx, projectID, console.ProjectDisabled)
		if err != nil {
			chore.log.Error("failed to mark project as disabled",
				zap.String("task", task),
				zap.String("public_project_id", projectPublicID.String()),
				zap.String("user_id", ownerID.String()),
				zap.Error(err),
			)
			return err
		}

		chore.log.Info("marked project as disabled",
			zap.String("task", task),
			zap.String("public_project_id", projectPublicID.String()),
			zap.String("user_id", ownerID.String()),
		)
		return nil
	})
}

func (chore *Chore) deactivateUser(ctx context.Context, userID uuid.UUID, freezeEventType *console.AccountFreezeEventType, task string) (err error) {
	err = chore.accounts.CreditCards().RemoveAll(ctx, userID)
	if err != nil {
		chore.log.Error("failed to remove user credit cards",
			zap.String("task", task),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		return err
	}

	return chore.store.WithTx(ctx, func(ctx context.Context, tx console.DBTx) error {
		_, err = tx.WebappSessions().DeleteAllByUserID(ctx, userID)
		if err != nil {
			chore.log.Error("failed to remove webapp sessions for user",
				zap.String("task", task),
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
			return err
		}

		deactivatedEmail := fmt.Sprintf("deactivated+%s@storj.io", userID.String())
		status := console.Deleted
		err = tx.Users().Update(ctx, userID, console.UpdateUserRequest{
			FullName:                    new(string),
			ShortName:                   new(*string),
			Email:                       &deactivatedEmail,
			Status:                      &status,
			ExternalID:                  new(*string),
			EmailChangeVerificationStep: new(int),
		})
		if err != nil {
			chore.log.Error("failed to update user status to Deleted",
				zap.String("task", task),
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
			return err
		}

		if freezeEventType != nil {
			err = tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, userID, *freezeEventType)
			if err != nil {
				chore.log.Error("failed to remove freeze event",
					zap.String("task", task),
					zap.String("user_id", userID.String()),
					zap.Error(err))
				return err
			}
		}

		chore.log.Info(
			"user deactivated",
			zap.String("task", task),
			zap.String("user_id", userID.String()),
		)

		return nil
	})
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
