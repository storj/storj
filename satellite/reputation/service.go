// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeevents"
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

// DirectDB is used only for mud, to differentiate between the interface (reputation.DB) and the two implementation (reputation.CachingDB and reputation.DirectDB).
// as the satellitedb.Reputation() doesn't return with any concrete type.
type DirectDB interface {
	DB
}

// Info contains all reputation data to be stored in DB.
type Info struct {
	AuditSuccessCount           int64
	TotalAuditCount             int64
	CreatedAt                   *time.Time
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
	i2.CreatedAt = cloneTime(i.CreatedAt)
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
	overlay *overlay.Service
	db      DB
	config  Config
}

// NewService creates a new reputation service.
func NewService(log *zap.Logger, overlay *overlay.Service, db DB, config Config) *Service {
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

	// There are some cases where the caller did not get updated reputation-status information.
	// (Usually this means the node was offline, disqualified, or exited and we skipped creating an order limit for it.)
	var nodeExited bool
	if reputation.Email == "" {
		dossier, err := service.overlay.Get(ctx, nodeID)
		if err != nil {
			return err
		}
		reputation = dossier.Reputation.Status
		if dossier.ExitStatus.ExitFinishedAt != nil {
			nodeExited = true
		}
	}

	// If the node is disqualified or exited, we do not need to apply the audit, so return nil.
	if reputation.Disqualified != nil || nodeExited {
		return nil
	}

	now := time.Now()
	statusUpdate, err := service.db.Update(ctx, UpdateRequest{
		NodeID:       nodeID,
		AuditOutcome: result,
		Config:       service.config,
	}, now)
	if err != nil {
		return err
	}

	// Only update node if its health status has changed, or the vetted information
	// is not set in the nodes table yet.
	changed, repChanges := hasReputationChanged(*statusUpdate, reputation)
	if changed {
		reputationUpdate := &overlay.ReputationUpdate{
			Disqualified:           statusUpdate.Disqualified,
			DisqualificationReason: statusUpdate.DisqualificationReason,
			UnknownAuditSuspended:  statusUpdate.UnknownAuditSuspended,
			OfflineSuspended:       statusUpdate.OfflineSuspended,
			VettedAt:               statusUpdate.VettedAt,
		}
		err = service.overlay.UpdateReputation(ctx, nodeID, reputation.Email, *reputationUpdate, repChanges)
		if err != nil {
			return errs.New("'nodes' table reputation updated failed: %+v. %w", reputationUpdate, err)
		}
	}

	return nil
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

	n, err := service.overlay.Get(ctx, nodeID)
	if err != nil {
		return err
	}

	update := overlay.ReputationUpdate{
		Disqualified:          n.Disqualified,
		UnknownAuditSuspended: &suspendedAt,
		OfflineSuspended:      n.OfflineSuspended,
		VettedAt:              n.Reputation.Status.VettedAt,
	}
	if n.DisqualificationReason != nil {
		update.DisqualificationReason = *n.DisqualificationReason
	}
	return service.overlay.UpdateReputation(ctx, nodeID, "", update, []nodeevents.Type{nodeevents.UnknownAuditSuspended})
}

// TestDisqualifyNode disqualifies a storage node.
func (service *Service) TestDisqualifyNode(ctx context.Context, nodeID storj.NodeID, reason overlay.DisqualificationReason) (err error) {
	disqualifiedAt := time.Now()

	err = service.db.DisqualifyNode(ctx, nodeID, disqualifiedAt, reason)
	if err != nil {
		return err
	}

	return service.overlay.DisqualifyNode(ctx, nodeID, reason)
}

// TestUnsuspendNodeUnknownAudit unsuspends a storage node for unknown audits.
func (service *Service) TestUnsuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID) (err error) {
	err = service.db.UnsuspendNodeUnknownAudit(ctx, nodeID)
	if err != nil {
		return err
	}

	n, err := service.overlay.Get(ctx, nodeID)
	if err != nil {
		return err
	}

	update := overlay.ReputationUpdate{
		Disqualified:          n.Disqualified,
		UnknownAuditSuspended: nil,
		OfflineSuspended:      n.OfflineSuspended,
		VettedAt:              n.Reputation.Status.VettedAt,
	}
	if n.DisqualificationReason != nil {
		update.DisqualificationReason = *n.DisqualificationReason
	}
	return service.overlay.UpdateReputation(ctx, nodeID, "", update, []nodeevents.Type{nodeevents.UnknownAuditUnsuspended})
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
func hasReputationChanged(updated Info, current overlay.ReputationStatus) (changed bool, repChanges []nodeevents.Type) {
	// there is no unDQ, so only update if changed from nil to not nil
	if current.Disqualified == nil && updated.Disqualified != nil {
		repChanges = append(repChanges, nodeevents.Disqualified)
		changed = true
	}
	if statusChanged(current.UnknownAuditSuspended, updated.UnknownAuditSuspended) {
		if updated.UnknownAuditSuspended != nil {
			repChanges = append(repChanges, nodeevents.UnknownAuditSuspended)
		} else {
			repChanges = append(repChanges, nodeevents.UnknownAuditUnsuspended)
		}
		changed = true
	}
	if statusChanged(current.OfflineSuspended, updated.OfflineSuspended) {
		if updated.OfflineSuspended != nil {
			repChanges = append(repChanges, nodeevents.OfflineSuspended)
		} else {
			repChanges = append(repChanges, nodeevents.OfflineUnsuspended)
		}
		changed = true
	}

	if updated.VettedAt != nil && current.VettedAt == nil {
		changed = true
	}
	return changed, repChanges
}

// statusChanged determines if the two given statuses are different.
// a status is considered "different" if it went from nil to not-nil, or not-nil to nil.
// if not-nil and the only difference is the time, this is considered "not changed".
func statusChanged(s1, s2 *time.Time) bool {
	return (s1 == nil && s2 != nil) || (s1 != nil && s2 == nil)
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
