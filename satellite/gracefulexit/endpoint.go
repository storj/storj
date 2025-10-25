// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
)

// millis for the transfer queue building ticker.
const buildQueueMillis = 100

var (
	// ErrInvalidArgument is an error class for invalid argument errors used to check which rpc code to use.
	ErrInvalidArgument = errs.Class("graceful exit")
	// ErrIneligibleNodeAge is an error class for when a node has not been on the network long enough to graceful exit.
	ErrIneligibleNodeAge = errs.Class("node is not yet eligible for graceful exit")
)

// Endpoint for handling the transfer of pieces for Graceful Exit.
type Endpoint struct {
	pb.DRPCSatelliteGracefulExitUnimplementedServer

	log            *zap.Logger
	interval       time.Duration
	signer         signing.Signer
	overlaydb      overlay.DB
	overlay        *overlay.Service
	reputation     *reputation.Service
	metabase       *metabase.DB
	orders         *orders.Service
	peerIdentities overlay.PeerIdentities
	config         Config

	nowFunc func() time.Time
}

// NewEndpoint creates a new graceful exit endpoint.
func NewEndpoint(log *zap.Logger, signer signing.Signer, overlaydb overlay.DB, overlay *overlay.Service, reputation *reputation.Service, metabase *metabase.DB, orders *orders.Service,
	peerIdentities overlay.PeerIdentities, config Config) *Endpoint {
	return &Endpoint{
		log:            log,
		interval:       time.Millisecond * buildQueueMillis,
		signer:         signer,
		overlaydb:      overlaydb,
		overlay:        overlay,
		reputation:     reputation,
		metabase:       metabase,
		orders:         orders,
		peerIdentities: peerIdentities,
		config:         config,
		nowFunc:        func() time.Time { return time.Now().UTC() },
	}
}

// SetNowFunc applies a function to be used in determining the "now" time for graceful exit
// purposes.
func (endpoint *Endpoint) SetNowFunc(timeFunc func() time.Time) {
	endpoint.nowFunc = timeFunc
}

// Process is called by storage nodes to receive pieces to transfer to new nodes and get exit status.
func (endpoint *Endpoint) Process(stream pb.DRPCSatelliteGracefulExit_ProcessStream) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Unauthenticated, Error.Wrap(err).Error())
	}

	endpoint.log.Debug("graceful exit process", zap.Stringer("Node ID", peer.ID))

	return endpoint.processTimeBased(ctx, stream, peer.ID)
}

func (endpoint *Endpoint) processTimeBased(ctx context.Context, stream pb.DRPCSatelliteGracefulExit_ProcessStream, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	nodeInfo, err := endpoint.overlay.Get(ctx, nodeID)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	isDisqualified, err := endpoint.handleDisqualifiedNode(ctx, nodeInfo)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	if isDisqualified {
		return rpcstatus.Error(rpcstatus.FailedPrecondition, "node is disqualified")
	}
	if endpoint.handleSuspendedNode(nodeInfo) {
		return rpcstatus.Error(rpcstatus.FailedPrecondition, "node is suspended. Please get node unsuspended before initiating graceful exit")
	}

	msg, err := endpoint.checkExitStatus(ctx, nodeInfo)
	if err != nil {
		if ErrIneligibleNodeAge.Has(err) {
			return rpcstatus.Error(rpcstatus.FailedPrecondition, err.Error())
		}
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	err = stream.Send(msg)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return nil
}

func (endpoint *Endpoint) handleDisqualifiedNode(ctx context.Context, nodeInfo *overlay.NodeDossier) (isDisqualified bool, err error) {
	if nodeInfo.Disqualified != nil {
		if nodeInfo.ExitStatus.ExitInitiatedAt == nil {
			// node never started graceful exit before, and it is already disqualified; nothing
			// for us to do here
			return true, nil
		}
		if nodeInfo.ExitStatus.ExitFinishedAt == nil {
			// node did start graceful exit and hasn't been marked as finished, although it
			// has been disqualified. We'll correct that now.
			exitStatusRequest := &overlay.ExitStatusRequest{
				NodeID:         nodeInfo.Id,
				ExitFinishedAt: endpoint.nowFunc(),
				ExitSuccess:    false,
			}

			_, err = endpoint.overlaydb.UpdateExitStatus(ctx, exitStatusRequest)
			return true, Error.Wrap(err)
		}
		return true, nil
	}
	return false, nil
}

func (endpoint *Endpoint) handleSuspendedNode(nodeInfo *overlay.NodeDossier) (isSuspended bool) {
	if nodeInfo.UnknownAuditSuspended != nil || nodeInfo.OfflineSuspended != nil {
		// If the node already initiated graceful exit, we'll let it carry on until / unless it gets disqualified.
		// Otherwise, the operator should make an effort to get the node un-suspended before initiating GE.
		// (The all-wise Go linter won't let me write this in a clearer way.)
		return nodeInfo.ExitStatus.ExitInitiatedAt == nil
	}
	return false
}

func (endpoint *Endpoint) getFinishedMessage(ctx context.Context, nodeID storj.NodeID, finishedAt time.Time, success bool, reason pb.ExitFailed_Reason) (message *pb.SatelliteMessage, err error) {
	if success {
		return endpoint.getFinishedSuccessMessage(ctx, nodeID, finishedAt)
	}
	return endpoint.getFinishedFailureMessage(ctx, nodeID, finishedAt, reason)
}

func (endpoint *Endpoint) getFinishedSuccessMessage(ctx context.Context, nodeID storj.NodeID, finishedAt time.Time) (message *pb.SatelliteMessage, err error) {
	unsigned := &pb.ExitCompleted{
		SatelliteId: endpoint.signer.ID(),
		NodeId:      nodeID,
		Completed:   finishedAt,
	}
	signed, err := signing.SignExitCompleted(ctx, endpoint.signer, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &pb.SatelliteMessage{Message: &pb.SatelliteMessage_ExitCompleted{
		ExitCompleted: signed,
	}}, nil
}

func (endpoint *Endpoint) getFinishedFailureMessage(ctx context.Context, nodeID storj.NodeID, finishedAt time.Time, reason pb.ExitFailed_Reason) (message *pb.SatelliteMessage, err error) {
	unsigned := &pb.ExitFailed{
		SatelliteId: endpoint.signer.ID(),
		NodeId:      nodeID,
		Failed:      finishedAt,
	}
	if reason >= 0 {
		unsigned.Reason = reason
	}
	signed, err := signing.SignExitFailed(ctx, endpoint.signer, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	message = &pb.SatelliteMessage{Message: &pb.SatelliteMessage_ExitFailed{
		ExitFailed: signed,
	}}
	return message, nil
}

func (endpoint *Endpoint) checkExitStatus(ctx context.Context, nodeInfo *overlay.NodeDossier) (*pb.SatelliteMessage, error) {
	if nodeInfo.ExitStatus.ExitFinishedAt != nil {
		// TODO maybe we should store the reason in the DB so we know how it originally failed.
		return endpoint.getFinishedMessage(ctx, nodeInfo.Id, *nodeInfo.ExitStatus.ExitFinishedAt, nodeInfo.ExitStatus.ExitSuccess, -1)
	}

	if nodeInfo.ExitStatus.ExitInitiatedAt == nil {
		// the node has just requested to begin GE. verify eligibility and set it up in the DB.
		geEligibilityDate := nodeInfo.CreatedAt.AddDate(0, endpoint.config.NodeMinAgeInMonths, 0)
		if endpoint.nowFunc().Before(geEligibilityDate) {
			return nil, ErrIneligibleNodeAge.New("will be eligible after %s", geEligibilityDate.String())
		}

		request := &overlay.ExitStatusRequest{
			NodeID:          nodeInfo.Id,
			ExitInitiatedAt: endpoint.nowFunc(),
		}
		node, err := endpoint.overlaydb.UpdateExitStatus(ctx, request)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		reputationInfo, err := endpoint.reputation.Get(ctx, nodeInfo.Id)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		// graceful exit initiation metrics
		age := endpoint.nowFunc().Sub(node.CreatedAt)
		mon.FloatVal("graceful_exit_init_node_age_seconds").Observe(age.Seconds())
		mon.IntVal("graceful_exit_init_node_audit_success_count").Observe(reputationInfo.AuditSuccessCount)
		mon.IntVal("graceful_exit_init_node_audit_total_count").Observe(reputationInfo.TotalAuditCount)
		mon.IntVal("graceful_exit_init_node_piece_count").Observe(node.PieceCount)
	} else {
		// the node has already initiated GE and hasn't finished yet... or has it?!?!
		geDoneDate := nodeInfo.ExitStatus.ExitInitiatedAt.AddDate(0, 0, endpoint.config.GracefulExitDurationInDays)
		if endpoint.nowFunc().After(geDoneDate) {
			// ok actually it has finished, and this is the first time we've noticed it
			reputationInfo, err := endpoint.reputation.Get(ctx, nodeInfo.Id)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			request := &overlay.ExitStatusRequest{
				NodeID:         nodeInfo.Id,
				ExitFinishedAt: endpoint.nowFunc(),
				ExitSuccess:    true,
			}
			var reason pb.ExitFailed_Reason

			// We don't check the online score constantly over the course of the graceful exit,
			// because we want to give the node a chance to get the score back up if it's
			// temporarily low.
			//
			// Instead, we check the overall score at the end of the GE period.
			if reputationInfo.OnlineScore < endpoint.config.MinimumOnlineScore {
				request.ExitSuccess = false
				reason = pb.ExitFailed_INACTIVE_TIMEFRAME_EXCEEDED
			}
			// If a node has lost all of its data, it could still initiate graceful exit and return
			// unknown errors to audits, getting suspended but not disqualified. Since such nodes
			// should not receive their held amount back, any nodes that are suspended at the end
			// of the graceful exit period will be treated as having failed graceful exit.
			if reputationInfo.UnknownAuditSuspended != nil {
				request.ExitSuccess = false
				reason = pb.ExitFailed_OVERALL_FAILURE_PERCENTAGE_EXCEEDED
			}
			endpoint.log.Info("node completed graceful exit",
				zap.Float64("online score", reputationInfo.OnlineScore),
				zap.Bool("suspended", reputationInfo.UnknownAuditSuspended != nil),
				zap.Bool("success", request.ExitSuccess),
				zap.Stringer("node ID", nodeInfo.Id))
			updatedNode, err := endpoint.overlaydb.UpdateExitStatus(ctx, request)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			if request.ExitSuccess {
				mon.Meter("graceful_exit_success").Mark(1)
				return endpoint.getFinishedSuccessMessage(ctx, updatedNode.Id, *updatedNode.ExitStatus.ExitFinishedAt)
			}
			mon.Meter("graceful_exit_failure").Mark(1)
			return endpoint.getFinishedFailureMessage(ctx, updatedNode.Id, *updatedNode.ExitStatus.ExitFinishedAt, reason)
		}
	}

	// this will cause the node to disconnect, wait a bit, and then try asking again.
	return &pb.SatelliteMessage{Message: &pb.SatelliteMessage_NotReady{NotReady: &pb.NotReady{}}}, nil
}

// GracefulExitFeasibility returns node's joined at date, nodeMinAge and if graceful exit available.
func (endpoint *Endpoint) GracefulExitFeasibility(ctx context.Context, req *pb.GracefulExitFeasibilityRequest) (_ *pb.GracefulExitFeasibilityResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, Error.Wrap(err).Error())
	}

	endpoint.log.Debug("graceful exit process", zap.Stringer("Node ID", peer.ID))

	var response pb.GracefulExitFeasibilityResponse

	nodeDossier, err := endpoint.overlay.Get(ctx, peer.ID)
	if err != nil {
		endpoint.log.Error("unable to retrieve node dossier for attempted exiting node", zap.Stringer("node ID", peer.ID))
		return nil, Error.Wrap(err)
	}

	eligibilityDate := nodeDossier.CreatedAt.AddDate(0, endpoint.config.NodeMinAgeInMonths, 0)
	if endpoint.nowFunc().Before(eligibilityDate) {
		response.IsAllowed = false
	} else {
		response.IsAllowed = true
	}

	response.JoinedAt = nodeDossier.CreatedAt
	response.MonthsRequired = int32(endpoint.config.NodeMinAgeInMonths)
	return &response, nil
}
