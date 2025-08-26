// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package pendingdelete

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
)

var (
	// Error defines the pendingdelete chore errors class.
	Error = errs.Class("pendingdelete")
	mon   = monkit.Package()
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
	log       *zap.Logger
	config    Config
	bucketsDB buckets.DB
	metabase  *metabase.DB
	store     console.DB

	nowFn func() time.Time

	Loop *sync2.Cycle
}

// NewChore creates a new instance of this chore.
func NewChore(log *zap.Logger, config Config,
	bucketsDB buckets.DB, consoleDB console.DB, metabase *metabase.DB,
) *Chore {
	return &Chore{
		log:       log,
		config:    config,
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

	return chore.Loop.Run(ctx, chore.runDeleteProjects)
}

func (chore *Chore) runDeleteProjects(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !chore.config.Project.Enabled {
		chore.log.Debug("skipping deleting pending deletion projects because it is disabled in config")
		return nil
	}

	mu := new(sync.Mutex)
	var errGrp errs.Group

	addErr := func(err error) {
		mu.Lock()
		errGrp.Add(err)
		mu.Unlock()
	}

	var processedProjects atomic.Int64
	hasNext := true
	for hasNext {
		idsPage, err := chore.store.Projects().ListPendingDeletionBefore(
			ctx,
			0, // always on offset 0 because updating project status removes it from the list
			chore.config.ListLimit,
			chore.nowFn().Add(-chore.config.Project.BufferTime),
		)
		if err != nil {
			chore.log.Error("failed to get projects for deletion", zap.Error(err))
			return err
		}
		hasNext = idsPage.Next

		if !hasNext && len(idsPage.Ids) == 0 {
			break
		}

		limiter := sync2.NewLimiter(chore.config.DeleteConcurrency)

		for _, p := range idsPage.Ids {
			limiter.Go(ctx, func() {
				err := chore.deleteData(ctx, p.ProjectID, p.OwnerID)
				if err != nil {
					addErr(err)
					return
				}

				err = chore.disableProject(ctx, p.ProjectID, p.OwnerID)
				if err != nil {
					addErr(err)
					return
				}
				processedProjects.Add(1)
			})
		}

		limiter.Wait()
	}

	chore.log.Info("finished deleting projects",
		zap.Int64("deleted_projects", processedProjects.Load()),
	)

	return Error.Wrap(errGrp.Err())
}

func (chore *Chore) deleteData(ctx context.Context, projectID, ownerID uuid.UUID) (err error) {
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
				zap.Int64("objectCount", objectCount),
				zap.String("userID", ownerID.String()),
				zap.String("projectID", projectID.String()),
				zap.String("bucket", bucket.Name),
			)
		}
	}

	return nil
}

func (chore *Chore) disableProject(ctx context.Context, projectID, ownerID uuid.UUID) (err error) {
	return chore.store.WithTx(ctx, func(ctx context.Context, tx console.DBTx) error {
		// delete project API keys.
		err = tx.APIKeys().DeleteAllByProjectID(ctx, projectID)
		if err != nil {
			chore.log.Error("failed to delete all API Keys for project",
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
				zap.String("projectID", projectID.String()),
				zap.String("userID", ownerID.String()),
				zap.Error(err),
			)
		}

		// disable the project.
		err = tx.Projects().UpdateStatus(ctx, projectID, console.ProjectDisabled)
		if err != nil {
			chore.log.Error("failed to mark project as disabled",
				zap.String("projectID", projectID.String()),
				zap.String("userID", ownerID.String()),
				zap.Error(err),
			)
			return err
		}

		chore.log.Info("marked project as disabled",
			zap.String("projectID", projectID.String()),
			zap.String("userID", ownerID.String()),
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
