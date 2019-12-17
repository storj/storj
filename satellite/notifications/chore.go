// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/private/sync2"
	"storj.io/storj/satellite/overlay"
)

type Chore struct {
	log     *zap.Logger
	dialer  rpc.Dialer
	overlay *overlay.Service
	Cycle   sync2.Cycle
}

func NewChore(log *zap.Logger, dialer rpc.Dialer, overlay *overlay.Service, interval time.Duration) *Chore {
	return &Chore{
		log:     log,
		dialer:  dialer,
		overlay: overlay,
		Cycle:   *sync2.NewCycle(interval),
	}
}

// Run runs notifications report cycle.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return chore.Cycle.Run(ctx,
		func(ctx context.Context) error {
			chore.log.Info("sending reports to storagenodes")

			if err := chore.report(ctx); err != nil {
				chore.log.Error("reporter cycle failed", zap.Error(err))
			}

			return nil
		},
	)
}

// Close closes underlying cycle.
func (chore *Chore) Close() (err error) {
	defer mon.Task()(nil)(&err)
	chore.Cycle.Close()
	return nil
}

// report sends reputation reports to all reliable nodes.
func (chore *Chore) report(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	sendReports := func(ctx context.Context, nodes []*overlay.NodeDossier) error {
		for _, node := range nodes {
			if err = ctx.Err(); err != nil {
				return err
			}

			chore.sendReport(ctx, node)
		}

		return nil
	}

	return chore.iterate(ctx, sendReports)
}

// iterate iterates through batches of reliable nodes.
func (chore *Chore) iterate(ctx context.Context, fn func(context.Context, []*overlay.NodeDossier) error) (err error) {
	defer mon.Task()(&ctx)(nil)

	var offset int64

	// use default lookup limit
	nodes, more, err := chore.overlay.PaginateReliable(ctx, offset, 0)
	if err != nil {
		return err
	}

	if err = fn(ctx, nodes); err != nil {
		return err
	}

	for more {
		if err = ctx.Err(); err != nil {
			return err
		}

		nodes, more, err = chore.overlay.PaginateReliable(ctx, offset+int64(len(nodes)), 0)
		if err != nil {
			return err
		}

		if err = fn(ctx, nodes); err != nil {
			return err
		}
	}

	return nil
}

// sendReport sends report for provided node.
func (chore *Chore) sendReport(ctx context.Context, node *overlay.NodeDossier) {
	defer mon.Task()(&ctx)(nil)

	logger := chore.log.Named(fmt.Sprintf("reporter %s", node.Id.String()))

	client, err := NewClient(ctx, chore.dialer, node.Id, node.GetAddress().GetAddress())
	if err != nil {
		logger.Debug("failed to establish connection", zap.Error(err))
		return
	}

	auditScore := calculateReputationScore(
		node.Reputation.AuditReputationAlpha,
		node.Reputation.AuditReputationBeta,
	)
	uptimeScore := calculateReputationScore(
		node.Reputation.UptimeReputationAlpha,
		node.Reputation.UptimeReputationBeta,
	)

	reportRequest := &pb.ReportRequest{
		AuditTotalCount:       node.Reputation.AuditCount,
		AuditSuccessCount:     node.Reputation.AuditSuccessCount,
		AuditReputationAlpha:  node.Reputation.AuditReputationAlpha,
		AuditReputationBeta:   node.Reputation.AuditReputationBeta,
		AuditReputationScore:  auditScore,
		UptimeTotalCount:      node.Reputation.UptimeCount,
		UptimeSuccessCount:    node.Reputation.UptimeSuccessCount,
		UptimeReputationAlpha: node.Reputation.UptimeReputationAlpha,
		UptimeReputationBeta:  node.Reputation.UptimeReputationBeta,
		UptimeReputationScore: uptimeScore,
		LastContactSuccess:    node.Reputation.LastContactSuccess,
		LastContactFailure:    node.Reputation.LastContactFailure,
		ExitLoopCompletedAt:   node.ExitStatus.ExitLoopCompletedAt,
		ExitInitiatedAt:       node.ExitStatus.ExitInitiatedAt,
		ExitFinishedAt:        node.ExitStatus.ExitFinishedAt,
		ExitSuccess:           node.ExitStatus.ExitSuccess,
		Vetted:                !chore.overlay.IsNew(node),
	}

	if _, err = client.Report(ctx, reportRequest); err != nil {
		logger.Debug("failed to send report", zap.Error(err))
		return
	}

	if err = client.Close(); err != nil {
		logger.Debug("failed to close connection", zap.Error(err))
		return
	}
}

// calculateReputationScore is helper method to calculate reputation score value.
func calculateReputationScore(alpha, beta float64) float64 {
	return alpha / (alpha + beta)
}
