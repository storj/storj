// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
)

var mon = monkit.Package()

// DB is an interface for storing reputation data.
type DB interface {
	Update(ctx context.Context, request UpdateRequest, now time.Time) (_ *overlay.ReputationStatus, changed bool, err error)
	Get(ctx context.Context, nodeID storj.NodeID) (*Info, error)
}

// Info contains all reputation data to be stored in DB.
type Info struct {
	AuditSuccessCount           int64
	TotalAuditCount             int64
	VettedAt                    *time.Time
	Contained                   bool
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
func (service *Service) ApplyAudit(ctx context.Context, nodeID storj.NodeID, result AuditType) (err error) {
	mon.Task()(&ctx)(&err)

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
	}

	return err
}

// Get returns a node's reputation info from DB.
func (service *Service) Get(ctx context.Context, nodeID storj.NodeID) (info *Info, err error) {
	mon.Task()(&ctx)(&err)
	return service.db.Get(ctx, nodeID)
}

// TestingSetState manually sets a node's info in DB.
func (service *Service) TestingSetState(state Info) error {
	return errs.New("reputation service method TestingSetState is NI")
}
