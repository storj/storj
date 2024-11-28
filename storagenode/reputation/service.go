// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/trust"
)

// Service is the reputation service.
//
// architecture: Service
type Service struct {
	log *zap.Logger

	db            DB
	nodeID        storj.NodeID
	notifications *notifications.Service

	dialer rpc.Dialer
	trust  *trust.Pool
}

// NewService creates new instance of service.
func NewService(log *zap.Logger, db DB, dialer rpc.Dialer, trust *trust.Pool, nodeID storj.NodeID, notifications *notifications.Service) *Service {
	return &Service{
		log:           log,
		db:            db,
		dialer:        dialer,
		trust:         trust,
		nodeID:        nodeID,
		notifications: notifications,
	}
}

// Store stores reputation stats into db, and notify's in case of offline suspension.
func (s *Service) Store(ctx context.Context, stats Stats, satelliteID storj.NodeID) error {
	rep, err := s.db.Get(ctx, satelliteID)
	if err != nil {
		return err
	}

	report := []zap.Field{
		zap.Stringer("Satellite ID", satelliteID),
		zap.Int64("Total Audits", stats.Audit.TotalCount),
		zap.Int64("Successful Audits", stats.Audit.SuccessCount),
		zap.Float64("Audit Score", stats.Audit.Score),
		zap.Float64("Online Score", stats.OnlineScore),
		zap.Float64("Suspension Score", stats.Audit.UnknownScore),
		zap.Float64("Audit Score Delta", stats.Audit.Score-rep.Audit.Score),
		zap.Float64("Online Score Delta", stats.OnlineScore-rep.OnlineScore),
		zap.Float64("Suspension Score Delta", stats.Audit.UnknownScore-rep.Audit.UnknownScore),
	}

	if stats.Audit.Score < rep.Audit.Score || stats.OnlineScore < rep.OnlineScore || stats.Audit.UnknownScore < rep.Audit.UnknownScore {
		s.log.Warn("node scores worsened", report...)
	} else {
		s.log.Info("node scores updated", report...)
	}

	err = s.db.Store(ctx, stats)
	if err != nil {
		return err
	}

	if stats.DisqualifiedAt == nil && isSuspended(stats, *rep) {
		notification := newSuspensionNotification(satelliteID, s.nodeID, *stats.OfflineSuspendedAt)

		_, err = s.notifications.Receive(ctx, notification)
		if err != nil {
			s.log.Sugar().Error("failed to receive notification", err.Error())
		}
	}

	return nil
}

// GetStats retrieves reputation stats from particular satellite.
func (s *Service) GetStats(ctx context.Context, satelliteID storj.NodeID) (_ *Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := s.dial(ctx, satelliteID)
	if err != nil {
		return nil, ErrReputationService.Wrap(err)
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	resp, err := client.GetStats(ctx, &pb.GetStatsRequest{})
	if err != nil {
		return nil, ErrReputationService.Wrap(err)
	}

	audit := resp.GetAuditCheck()

	satelliteIDSeriesTag := monkit.NewSeriesTag("satellite_id", satelliteID.String())

	mon.IntVal("audit_success_count", satelliteIDSeriesTag).Observe(audit.GetSuccessCount())
	mon.IntVal("audit_total_count", satelliteIDSeriesTag).Observe(audit.GetTotalCount())
	mon.FloatVal("audit_reputation_score", satelliteIDSeriesTag).Observe(audit.GetReputationScore())
	mon.FloatVal("suspension_score", satelliteIDSeriesTag).Observe(audit.GetUnknownReputationScore())
	mon.FloatVal("online_score", satelliteIDSeriesTag).Observe(resp.GetOnlineScore())

	return &Stats{
		SatelliteID: satelliteID,
		Audit: Metric{
			TotalCount:   audit.GetTotalCount(),
			SuccessCount: audit.GetSuccessCount(),
			Alpha:        audit.GetReputationAlpha(),
			Beta:         audit.GetReputationBeta(),
			Score:        audit.GetReputationScore(),
			UnknownAlpha: audit.GetUnknownReputationAlpha(),
			UnknownBeta:  audit.GetUnknownReputationBeta(),
			UnknownScore: audit.GetUnknownReputationScore(),
		},
		OnlineScore:          resp.OnlineScore,
		DisqualifiedAt:       resp.GetDisqualified(),
		SuspendedAt:          resp.GetSuspended(),
		OfflineSuspendedAt:   resp.GetOfflineSuspended(),
		OfflineUnderReviewAt: resp.GetOfflineUnderReview(),
		VettedAt:             resp.GetVettedAt(),
		AuditHistory:         resp.GetAuditHistory(),
		UpdatedAt:            time.Now(),
		JoinedAt:             resp.JoinedAt,
	}, nil
}

// Client encapsulates NodeStatsClient with underlying connection.
//
// architecture: Client
type Client struct {
	conn *rpc.Conn
	pb.DRPCNodeStatsClient
}

// Close closes underlying client connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// dial dials the NodeStats client for the satellite by id.
func (s *Service) dial(ctx context.Context, satelliteID storj.NodeID) (_ *Client, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeurl, err := s.trust.GetNodeURL(ctx, satelliteID)
	if err != nil {
		return nil, errs.New("unable to find satellite %s: %w", satelliteID, err)
	}

	conn, err := s.dialer.DialNodeURL(ctx, nodeurl)
	if err != nil {
		return nil, errs.New("unable to connect to the satellite %s: %w", satelliteID, err)
	}

	return &Client{
		conn:                conn,
		DRPCNodeStatsClient: pb.NewDRPCNodeStatsClient(conn),
	}, nil
}

// isSuspended returns if there's new downtime suspension.
func isSuspended(new, old Stats) bool {
	if new.OfflineSuspendedAt == nil {
		return false
	}

	if old.OfflineSuspendedAt == nil {
		return true
	}

	if !old.OfflineSuspendedAt.Equal(*new.OfflineSuspendedAt) {
		return true
	}

	return false
}

// newSuspensionNotification - returns offline suspension notification.
func newSuspensionNotification(satelliteID storj.NodeID, senderID storj.NodeID, time time.Time) (_ notifications.NewNotification) {
	return notifications.NewNotification{
		SenderID: senderID,
		Type:     notifications.TypeSuspension,
		Title:    "Your Node is suspended since " + time.String(),
		Message:  "This is a reminder that your StorageNode is suspended on Satellite " + satelliteID.String(),
	}
}
