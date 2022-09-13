// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
)

// DB is an interface for storing reputation data.
type DB interface {
	Update(ctx context.Context, request UpdateRequest, now time.Time) (_ *Info, err error)
	Get(ctx context.Context, nodeID storj.NodeID) (*Info, error)
	// ApplyUpdates applies multiple updates (defined by the updates
	// parameter) to a node's reputations record.
	ApplyUpdates(ctx context.Context, nodeID storj.NodeID, updates Mutations, reputationConfig Config, now time.Time) (_ *Info, err error)

	// UnsuspendNodeUnknownAudit unsuspends a storage node for unknown audits.
	UnsuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID) (err error)
	// DisqualifyNode disqualifies a storage node.
	DisqualifyNode(ctx context.Context, nodeID storj.NodeID, disqualifiedAt time.Time, reason overlay.DisqualificationReason) (err error)
	// SuspendNodeUnknownAudit suspends a storage node for unknown audits.
	SuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID, suspendedAt time.Time) (err error)
}

// Info contains all reputation data to be stored in DB.
type Info struct {
	AuditSuccessCount           int64
	TotalAuditCount             int64
	VettedAt                    *time.Time
	UnknownAuditSuspended       *time.Time
	OfflineSuspended            *time.Time
	UnderReview                 *time.Time
	Disqualified                *time.Time
	DisqualificationReason      overlay.DisqualificationReason
	OnlineScore                 float64
	AuditHistory                *pb.AuditHistory
	AuditReputationAlpha        float64
	AuditReputationBeta         float64
	UnknownAuditReputationAlpha float64
	UnknownAuditReputationBeta  float64
}

// Copy creates a deep copy of the Info object.
func (i *Info) Copy() *Info {
	i2 := *i
	i2.VettedAt = cloneTime(i.VettedAt)
	i2.UnknownAuditSuspended = cloneTime(i.UnknownAuditSuspended)
	i2.OfflineSuspended = cloneTime(i.OfflineSuspended)
	i2.UnderReview = cloneTime(i.UnderReview)
	i2.Disqualified = cloneTime(i.Disqualified)
	if i.AuditHistory != nil {
		i2.AuditHistory = &pb.AuditHistory{
			Score:   i.AuditHistory.Score,
			Windows: make([]*pb.AuditWindow, len(i.AuditHistory.Windows)),
		}
		for i, window := range i.AuditHistory.Windows {
			w := *window
			i2.AuditHistory.Windows[i] = &w
		}
	}
	return &i2
}

// Mutations represents changes which should be made to a particular node's
// reputation, in terms of counts and/or timestamps of events which have
// occurred. A Mutations record can be applied to a reputations row without
// prior knowledge of that row's contents.
type Mutations struct {
	PositiveResults int
	FailureResults  int
	UnknownResults  int
	OfflineResults  int
	OnlineHistory   *pb.AuditHistory
}

// Service handles storing node reputation data and updating
// the overlay cache when a node's status changes.
type Service struct {
	log     *zap.Logger
	overlay overlay.DB
	db      DB
	config  Config
}

// NewService creates a new reputation service.
func NewService(log *zap.Logger, overlay overlay.DB, db DB, config Config) *Service {
	return &Service{
		log:     log,
		overlay: overlay,
		db:      db,
		config:  config,
	}
}

// ApplyAudit receives an audit result and applies it to the relevant node in DB.
func (service *Service) ApplyAudit(ctx context.Context, nodeID storj.NodeID, reputation overlay.ReputationStatus, result AuditType) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now()
	statusUpdate, err := service.db.Update(ctx, UpdateRequest{
		NodeID:       nodeID,
		AuditOutcome: result,
		Config:       service.config,
	}, now)
	if err != nil {
		return err
	}

	// only update node if its health status has changed, or it's a newly vetted
	// node.
	// this prevents the need to require caller of ApplyAudit() to always know
	// the previous VettedAt time for a node.
	// Due to inconsistencies in the precision of time.Now() on different platforms and databases, the time comparison
	// for the VettedAt status is done using time values that are truncated to second precision.
	if hasReputationChanged(*statusUpdate, reputation, now) {
		reputationUpdate := &overlay.ReputationUpdate{
			Disqualified:           statusUpdate.Disqualified,
			DisqualificationReason: statusUpdate.DisqualificationReason,
			UnknownAuditSuspended:  statusUpdate.UnknownAuditSuspended,
			OfflineSuspended:       statusUpdate.OfflineSuspended,
			VettedAt:               statusUpdate.VettedAt,
		}
		err = service.overlay.UpdateReputation(ctx, nodeID, *reputationUpdate)
		if err != nil {
			return err
		}
	}

	return err
}

// Get returns a node's reputation info from DB.
// If a node is not found in the DB, default reputation information is returned.
func (service *Service) Get(ctx context.Context, nodeID storj.NodeID) (info *Info, err error) {
	defer mon.Task()(&ctx)(&err)

	info, err = service.db.Get(ctx, nodeID)
	if err != nil {
		if ErrNodeNotFound.Has(err) {
			// if there is no audit reputation for the node, that's fine and we
			// return default reputation values
			info = &Info{
				UnknownAuditReputationAlpha: 1,
				AuditReputationAlpha:        service.config.InitialAlpha,
				AuditReputationBeta:         service.config.InitialBeta,
				OnlineScore:                 1,
			}

			return info, nil
		}

		return nil, Error.Wrap(err)
	}

	return info, nil
}

// TestSuspendNodeUnknownAudit suspends a storage node for unknown audits.
func (service *Service) TestSuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID, suspendedAt time.Time) (err error) {
	err = service.db.SuspendNodeUnknownAudit(ctx, nodeID, suspendedAt)
	if err != nil {
		return err
	}

	return service.overlay.TestSuspendNodeUnknownAudit(ctx, nodeID, suspendedAt)
}

// TestDisqualifyNode disqualifies a storage node.
func (service *Service) TestDisqualifyNode(ctx context.Context, nodeID storj.NodeID, reason overlay.DisqualificationReason) (err error) {
	disqualifiedAt := time.Now()

	err = service.db.DisqualifyNode(ctx, nodeID, disqualifiedAt, reason)
	if err != nil {
		return err
	}

	return service.overlay.DisqualifyNode(ctx, nodeID, disqualifiedAt, reason)
}

// TestUnsuspendNodeUnknownAudit unsuspends a storage node for unknown audits.
func (service *Service) TestUnsuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID) (err error) {
	err = service.db.UnsuspendNodeUnknownAudit(ctx, nodeID)
	if err != nil {
		return err
	}

	return service.overlay.TestUnsuspendNodeUnknownAudit(ctx, nodeID)
}

// TestFlushAllNodeInfo flushes any and all cached information about all
// nodes to the backing store, if the attached reputationDB does any caching
// at all.
func (service *Service) TestFlushAllNodeInfo(ctx context.Context) (err error) {
	if db, ok := service.db.(*CachingDB); ok {
		return db.FlushAll(ctx)
	}
	return nil
}

// FlushNodeInfo flushes any cached information about the specified node to
// the backing store, if the attached reputationDB does any caching at all.
func (service *Service) FlushNodeInfo(ctx context.Context, nodeID storj.NodeID) (err error) {
	if db, ok := service.db.(*CachingDB); ok {
		return db.RequestSync(ctx, nodeID)
	}
	return nil
}

// Close closes resources.
func (service *Service) Close() error { return nil }

// hasReputationChanged determines if the current node reputation is different from the newly updated reputation. This
// function will only consider the Disqualified, UnknownAudiSuspended and OfflineSuspended statuses for changes.
func hasReputationChanged(updated Info, current overlay.ReputationStatus, now time.Time) bool {
	if statusChanged(current.Disqualified, updated.Disqualified) ||
		statusChanged(current.UnknownAuditSuspended, updated.UnknownAuditSuspended) ||
		statusChanged(current.OfflineSuspended, updated.OfflineSuspended) {
		return true
	}
	// check for newly vetted nodes.
	// Due to inconsistencies in the precision of time.Now() on different platforms and databases, the time comparison
	// for the VettedAt status is done using time values that are truncated to second precision.
	if updated.VettedAt != nil && updated.VettedAt.Truncate(time.Second).Equal(now.Truncate(time.Second)) {
		return true
	}
	return false
}

// statusChanged determines if the two given statuses are different.
func statusChanged(s1, s2 *time.Time) bool {
	if s1 == nil && s2 == nil {
		return false
	} else if s1 != nil && s2 != nil {
		return !s1.Equal(*s1)
	}
	return true
}

// UpdateRequestToMutations transforms an UpdateRequest into the equivalent
// Mutations structure, which can be used with ApplyUpdates.
func UpdateRequestToMutations(updateReq UpdateRequest, now time.Time) (Mutations, error) {
	updates := Mutations{}
	switch updateReq.AuditOutcome {
	case AuditSuccess:
		updates.PositiveResults = 1
	case AuditFailure:
		updates.FailureResults = 1
	case AuditUnknown:
		updates.UnknownResults = 1
	case AuditOffline:
		updates.OfflineResults = 1
	}
	updates.OnlineHistory = &pb.AuditHistory{}
	err := AddAuditToHistory(updates.OnlineHistory, updateReq.AuditOutcome != AuditOffline, now, updateReq.Config.AuditHistory)
	return updates, err
}

func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	tCopy := *t
	return &tCopy
}
