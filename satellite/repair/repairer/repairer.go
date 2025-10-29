// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"

	"storj.io/common/context2"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/queue"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("repairer")
	mon   = monkit.Package()
)

// Config contains configurable values for repairer.
type Config struct {
	MaxRepair                     int           `help:"maximum segments that can be repaired concurrently" releaseDefault:"5" devDefault:"1" testDefault:"10"`
	SegmentsSelectBatchSize       int           `help:"how many injured segments will be read from repair queue in single request" releaseDefault:"1" devDefault:"10"`
	Interval                      time.Duration `help:"how frequently repairer should try and repair more data" releaseDefault:"5m0s" devDefault:"1m0s" testDefault:"$TESTINTERVAL"`
	DialTimeout                   time.Duration `help:"time limit for dialing storage node" default:"5s"`
	Timeout                       time.Duration `help:"time limit for uploading repaired pieces to new storage nodes" default:"5m0s" testDefault:"1m"`
	DownloadTimeout               time.Duration `help:"time limit for downloading pieces from a node for repair" default:"5m0s" testDefault:"1m"`
	DownloadLongTail              int           `help:"number of extra concurrent downloads beyond required count for faster repairs" default:"0" testDefault:"3"`
	TotalTimeout                  time.Duration `help:"time limit for an entire repair job, from queue pop to upload completion" default:"45m" testDefault:"10m"`
	MaxExcessRateOptimalThreshold float64       `help:"ratio applied to the optimal threshold to calculate the excess of the maximum number of repaired pieces to upload" default:"0.05"`
	InMemoryRepair                bool          `help:"whether to download pieces for repair in memory (true) or download to disk (false)" default:"false"`
	InMemoryUpload                bool          `help:"whether to upload pieces for repair using memory (true) or disk (false)" default:"false"`
	ReputationUpdateEnabled       bool          `help:"whether the audit score of nodes should be updated as a part of repair" default:"false"`
	UseRangedLoop                 bool          `help:"whether to enable repair checker observer with ranged loop" default:"true"`
	RepairExcludedCountryCodes    []string      `help:"list of country codes to treat node from this country as offline" default:"" hidden:"true"`
	DoDeclumping                  bool          `help:"repair pieces on the same network to other nodes" default:"true"`
	DoPlacementCheck              bool          `help:"repair pieces out of segment placement" default:"true"`
	OnlineWindow                  time.Duration `help:"the amount of time without seeing a node before its considered offline" default:"4h" testDefault:"5m"`

	ParticipatingNodeCacheEnabled  bool          `help:"enable cache for participating nodes" default:"false" testDefault:"true"`
	ParticipatingNodeCacheInterval time.Duration `help:"how often participating nodes cache should be refreshed" default:"5m"`
	ParticipatingNodeCacheStale    time.Duration `help:"when does participating nodes cache start blocking" default:"10m"`

	NodesForRepairCacheEnabled  bool          `help:"enable cache for nodes to upload repaired pieces" default:"false" testDefault:"true"`
	NodesForRepairCacheInterval time.Duration `help:"how often nodes for repaired pieces cache should be refreshed" default:"5m"`
	NodesForRepairCacheStale    time.Duration `help:"when does nodes for repaired pieces cache start blocking" default:"10m"`

	IncludedPlacements PlacementList `help:"comma separated placement IDs (numbers), which should checked by the repairer (other placements are ignored)" default:""`
	ExcludedPlacements PlacementList `help:"comma separated placement IDs (numbers), placements which should be ignored by the repairer" default:""`
}

// Overlay is used to fetch information about nodes for repairing segments.
type Overlay interface {
	// GetOnlineNodesForRepair returns a map of nodes for the supplied nodeIDs.
	// The passed onlineWindow is used to determine whether each node is marked as Online.
	GetOnlineNodesForRepair(
		_ context.Context, _ []storj.NodeID, onlineWindow time.Duration,
	) (map[storj.NodeID]*overlay.NodeReputation, error)
	// FindStorageNodesForUpload searches the for nodes in the cache that meet the provided requirements for upload.
	FindStorageNodesForUpload(context.Context, overlay.FindStorageNodesRequest) ([]*nodeselection.SelectedNode, error)
	// GetParticipatingNodesForRepair returns all known participating nodes (this includes all known
	// nodes excluding nodes that have been disqualified or gracefully exited).
	// The passed onlineWindow is used to determine whether each node is marked as Online.
	// The results are returned in a slice of the same length as the input nodeIDs,
	// and each index of the returned list corresponds to the same index in nodeIDs.
	// If a node is not known, or is disqualified or exited, the corresponding returned SelectedNode
	// will have a zero value.
	GetParticipatingNodesForRepair(
		_ context.Context, _ storj.NodeIDList, onlineWindow time.Duration,
	) ([]nodeselection.SelectedNode, error)
	// GetAllParticipatingNodesForRepair returns all known participating nodes (this includes all known
	// nodes excluding nodes that have been disqualified or gracefully exited).
	// The passed onlineWindow is used to determine whether each node is marked as Online.
	GetAllParticipatingNodesForRepair(
		_ context.Context, onlineWindow time.Duration) ([]nodeselection.SelectedNode, error)
}

// PlacementList is a configurable, comma separated list of PlacementConstraint IDs.
type PlacementList struct {
	Placements []storj.PlacementConstraint
}

// String implements pflag.Value.
func (p *PlacementList) String() string {
	var s []string
	for _, pl := range p.Placements {
		s = append(s, fmt.Sprintf("%d", pl))
	}
	return strings.Join(s, ",")
}

// Set implements pflag.Value.
func (p *PlacementList) Set(s string) error {
	parts := strings.Split(s, ",")
	for _, pNumStr := range parts {
		pNumStr = strings.TrimSpace(pNumStr)
		if pNumStr == "" {
			continue
		}
		pNum, err := strconv.Atoi(pNumStr)
		if err != nil {
			return errs.New("Placement list should contain numbers: %s", s)
		}
		p.Placements = append(p.Placements, storj.PlacementConstraint(pNum))
	}
	return nil
}

// Type implements pflag.Value.
func (p PlacementList) Type() string {
	return "placement-list"
}

var _ pflag.Value = &PlacementList{}

// Service contains the information needed to run the repair service.
//
// architecture: Worker
type Service struct {
	log        *zap.Logger
	queue      queue.RepairQueue
	config     *Config
	JobLimiter *semaphore.Weighted
	Loop       *sync2.Cycle
	repairer   *SegmentRepairer

	nowFn func() time.Time
}

// NewService creates repairing service.
func NewService(log *zap.Logger, queue queue.RepairQueue, config *Config, repairer *SegmentRepairer) *Service {
	return &Service{
		log:        log,
		queue:      queue,
		config:     config,
		JobLimiter: semaphore.NewWeighted(int64(config.MaxRepair)),
		Loop:       sync2.NewCycle(config.Interval),
		repairer:   repairer,

		nowFn: time.Now,
	}
}

// Close closes resources.
func (service *Service) Close() error { return nil }

// WaitForPendingRepairs waits for all ongoing repairs to complete.
//
// NB: this assumes that service.config.MaxRepair will never be changed once this Service instance
// is initialized. If that is not a valid assumption, we should keep a copy of its initial value to
// use here instead.
func (service *Service) WaitForPendingRepairs(ctx context.Context) error {
	// Acquire and then release the entire capacity of the semaphore, ensuring that
	// it is completely empty (or, at least it was empty at some point).
	//
	// No error return is possible here; context.Background() can't be canceled
	err := service.JobLimiter.Acquire(ctx, int64(service.config.MaxRepair))
	if err != nil {
		return err
	}
	service.JobLimiter.Release(int64(service.config.MaxRepair))
	return nil
}

// TestingSetMinFailures sets the minFailures attribute, which tells the Repair machinery that we _expect_
// there to be failures and that we should wait for them if necessary. This is only used in tests.
func (service *Service) TestingSetMinFailures(minFailures int) {
	service.repairer.ec.TestingSetMinFailures(minFailures)
}

// Run runs the repairer service.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Wait for all repairs to complete
	defer func() {
		_ = service.WaitForPendingRepairs(context2.WithoutCancellation(ctx))
	}()

	return service.Loop.Run(ctx, service.processWhileQueueHasItems)
}

// processWhileQueueHasItems keeps calling process() until the queue is empty or something
// else goes wrong in fetching from the queue.
func (service *Service) processWhileQueueHasItems(ctx context.Context) error {
	for {
		err := service.process(ctx)
		if err != nil {
			if queue.ErrEmpty.Has(err) {
				return nil
			}
			service.log.Error("process", zap.Error(Error.Wrap(err)))
			return err
		}
	}
}

// process picks items from repair queue and spawns a repair worker.
func (service *Service) process(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	selectCtx, selectCancel := context.WithTimeout(ctx, service.config.TotalTimeout)
	defer selectCancel()
	segments, err := service.queue.Select(selectCtx, service.config.SegmentsSelectBatchSize,
		service.config.IncludedPlacements.Placements, service.config.ExcludedPlacements.Placements)
	if err != nil {
		return err
	}

	for _, seg := range segments {
		seg := seg
		// wait until we are allowed to spawn a new job
		if err := service.JobLimiter.Acquire(ctx, 1); err != nil {
			return err
		}

		// IMPORTANT: this timeout must be started before service.queue.Select(), in case
		// service.queue.Select() takes some non-negligible amount of time, so that we can depend on
		// repair jobs being given up within some set interval after the time in the 'attempted'
		// column in the queue table.
		//
		// This is the reason why we are using a semaphore in this somewhat awkward way instead of
		// using a simpler sync2.Limiter pattern. We don't want this timeout to include the waiting
		// time from the semaphore acquisition, but it _must_ include the queue fetch time. At the
		// same time, we don't want to do the queue pop in a separate goroutine, because we want to
		// return from service.Run when queue fetch fails.
		ctx, cancel := context.WithTimeout(ctx, service.config.TotalTimeout)

		log := service.log.With(zap.Stringer("Stream ID", seg.StreamID), zap.Uint64("Position", seg.Position.Encode()))
		log.Debug("Retrieved segment from repair queue")

		// this goroutine inherits the JobLimiter semaphore acquisition and is now responsible
		// for releasing it.
		go func() {
			defer service.JobLimiter.Release(1)
			defer cancel()
			if err := service.worker(ctx, seg); err != nil {
				log.Error("repair worker failed", zap.Uint16("placement", uint16(seg.Placement)), zap.Error(err))
			}
		}()
	}
	return nil
}

func (service *Service) worker(ctx context.Context, seg queue.InjuredSegment) (err error) {
	defer mon.Task()(&ctx)(&err)

	workerStartTime := service.nowFn().UTC()

	log := service.log.With(
		zap.Stringer("Stream ID", seg.StreamID),
		zap.Uint64("Position", seg.Position.Encode()))
	log.Debug("Limiter running repair on segment")

	// note that shouldDelete is used even in the case where err is not null
	shouldDelete, err := service.repairer.Repair(ctx, seg)
	if shouldDelete {
		if err != nil {
			log.Error("unexpected error repairing segment!", zap.Error(err))
		} else {
			log.Debug("removing repaired segment from repair queue")
		}

		delErr := service.queue.Release(ctx, seg, true)
		if delErr != nil {
			err = errs.Combine(err, Error.New("failed to remove segment from queue: %v", delErr))
		}
	} else {
		doneErr := service.queue.Release(ctx, seg, false)
		if doneErr != nil {
			err = errs.Combine(err, Error.New("failed to update segment in queue: %v", doneErr))
		}
	}
	if err != nil {
		return Error.Wrap(err)
	}

	placementTag := monkit.NewSeriesTag("placement", strconv.Itoa(int(seg.Placement)))
	repairedTime := service.nowFn().UTC()
	timeForRepair := repairedTime.Sub(workerStartTime)
	mon.FloatVal("time_for_repair", placementTag).Observe(timeForRepair.Seconds())

	insertedTime := seg.InsertedAt
	// do not send metrics if segment was added before the InsertedTime field was added
	if !insertedTime.IsZero() {
		timeSinceQueued := workerStartTime.Sub(insertedTime)
		mon.FloatVal("time_since_checker_queue", placementTag).Observe(timeSinceQueued.Seconds())
	}

	return nil
}

// SetNow allows tests to have the server act as if the current time is whatever they want.
func (service *Service) SetNow(nowFn func() time.Time) {
	service.nowFn = nowFn
	service.repairer.SetNow(nowFn)
}
