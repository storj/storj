// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
)

// DB is an interface for storing reputation data.
type DB interface {
	Update(ctx context.Context, request UpdateRequest, now time.Time) (_ *overlay.ReputationStatus, changed bool, err error)
	SetNodeStatus(ctx context.Context, id storj.NodeID, status overlay.ReputationStatus) error
	Get(ctx context.Context, nodeID storj.NodeID) (*Info, error)

	// UnsuspendNodeUnknownAudit unsuspends a storage node for unknown audits.
	UnsuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID) (err error)
	// DisqualifyNode disqualifies a storage node.
	DisqualifyNode(ctx context.Context, nodeID storj.NodeID) (err error)
	// SuspendNodeUnknownAudit suspends a storage node for unknown audits.
	SuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID, suspendedAt time.Time) (err error)
	// UpdateAuditHistory updates a node's audit history
	UpdateAuditHistory(ctx context.Context, oldHistory []byte, updateReq UpdateRequest, auditTime time.Time) (res *UpdateAuditHistoryResponse, err error)
}

// Info contains all reputation data to be stored in DB.
type Info struct {
	AuditSuccessCount           int64
	TotalAuditCount             int64
	VettedAt                    *time.Time
	Disqualified                *time.Time
	Suspended                   *time.Time
	UnknownAuditSuspended       *time.Time
	OfflineSuspended            *time.Time
	UnderReview                 *time.Time
	OnlineScore                 float64
	AuditHistory                AuditHistory
	AuditReputationAlpha        float64
	AuditReputationBeta         float64
	UnknownAuditReputationAlpha float64
	UnknownAuditReputationBeta  float64
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
func (service *Service) ApplyAudit(ctx context.Context, nodeID storj.NodeID, result AuditType) (err error) {
	defer mon.Task()(&ctx)(&err)

	statusUpdate, changed, err := service.db.Update(ctx, UpdateRequest{
		NodeID:       nodeID,
		AuditOutcome: result,

		AuditLambda:              service.config.AuditLambda,
		AuditWeight:              service.config.AuditWeight,
		AuditDQ:                  service.config.AuditDQ,
		SuspensionGracePeriod:    service.config.SuspensionGracePeriod,
		SuspensionDQEnabled:      service.config.SuspensionDQEnabled,
		AuditsRequiredForVetting: service.config.AuditCount,
		AuditHistory:             service.config.AuditHistory,
	}, time.Now())
	if err != nil {
		return err
	}

	if changed {
		err = service.overlay.UpdateReputation(ctx, nodeID, statusUpdate)
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
				AuditReputationAlpha:        1,
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
func (service *Service) TestDisqualifyNode(ctx context.Context, nodeID storj.NodeID) (err error) {
	err = service.db.DisqualifyNode(ctx, nodeID)
	if err != nil {
		return err
	}

	return service.overlay.DisqualifyNode(ctx, nodeID)
}

// TestUnsuspendNodeUnknownAudit unsuspends a storage node for unknown audits.
func (service *Service) TestUnsuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID) (err error) {
	err = service.db.UnsuspendNodeUnknownAudit(ctx, nodeID)
	if err != nil {
		return err
	}

	return service.overlay.TestUnsuspendNodeUnknownAudit(ctx, nodeID)
}

// Close closes resources.
func (service *Service) Close() error { return nil }
