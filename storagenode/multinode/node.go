// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/version"
	"storj.io/storj/private/multinodepb"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/operator"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/trust"
)

// ensures that NodeEndpoint implements multinodepb.DRPCNodeServer.
var _ multinodepb.DRPCNodeServer = (*NodeEndpoint)(nil)

// NodeEndpoint implements multinode node endpoint.
//
// architecture: Endpoint
type NodeEndpoint struct {
	multinodepb.DRPCNodeUnimplementedServer

	config operator.Config

	log        *zap.Logger
	apiKeys    *apikeys.Service
	version    version.Info
	contact    *contact.PingStats
	reputation reputation.DB
	trust      *trust.Pool
}

// NewNodeEndpoint creates new multinode node endpoint.
func NewNodeEndpoint(log *zap.Logger, config operator.Config, apiKeys *apikeys.Service, version version.Info, contact *contact.PingStats, reputation reputation.DB, trust *trust.Pool) *NodeEndpoint {
	return &NodeEndpoint{
		log:        log,
		config:     config,
		apiKeys:    apiKeys,
		version:    version,
		contact:    contact,
		reputation: reputation,
		trust:      trust,
	}
}

// Version returns node current version.
func (node *NodeEndpoint) Version(ctx context.Context, req *multinodepb.VersionRequest) (_ *multinodepb.VersionResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, node.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	return &multinodepb.VersionResponse{
		Version: node.version.Version.String(),
	}, nil
}

// LastContact returns timestamp when node was last in contact with satellite.
func (node *NodeEndpoint) LastContact(ctx context.Context, req *multinodepb.LastContactRequest) (_ *multinodepb.LastContactResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, node.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	return &multinodepb.LastContactResponse{
		LastContact: node.contact.WhenLastPinged(),
	}, nil
}

// Reputation returns reputation for specific satellite.
func (node *NodeEndpoint) Reputation(ctx context.Context, req *multinodepb.ReputationRequest) (_ *multinodepb.ReputationResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, node.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	rep, err := node.reputation.Get(ctx, req.SatelliteId)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}
	if rep.JoinedAt.IsZero() {
		return nil, rpcstatus.Error(rpcstatus.NotFound, "satellite reputation not found")
	}

	var windows []*multinodepb.AuditWindow
	if rep.AuditHistory != nil {
		for _, window := range rep.AuditHistory.Windows {
			windows = append(windows, &multinodepb.AuditWindow{
				WindowStart: window.WindowStart,
				OnlineCount: window.OnlineCount,
				TotalCount:  window.TotalCount,
			})
		}
	}

	return &multinodepb.ReputationResponse{
		Online: &multinodepb.ReputationResponse_Online{
			Score: rep.OnlineScore,
		},
		Audit: &multinodepb.ReputationResponse_Audit{
			Score:           rep.Audit.Score,
			SuspensionScore: rep.Audit.UnknownScore,
			TotalCount:      rep.Audit.TotalCount,
			SuccessCount:    rep.Audit.SuccessCount,
			Alpha:           rep.Audit.Alpha,
			Beta:            rep.Audit.Beta,
			UnknownAlpha:    rep.Audit.UnknownAlpha,
			UnknownBeta:     rep.Audit.UnknownBeta,
			History:         windows,
		},
		DisqualifiedAt:       rep.DisqualifiedAt,
		SuspendedAt:          rep.SuspendedAt,
		OfflineSuspendedAt:   rep.OfflineSuspendedAt,
		OfflineUnderReviewAt: rep.OfflineUnderReviewAt,
		VettedAt:             rep.VettedAt,
		UpdatedAt:            rep.UpdatedAt,
		JoinedAt:             rep.JoinedAt,
	}, nil
}

// TrustedSatellites returns list of trusted satellites node urls.
func (node *NodeEndpoint) TrustedSatellites(ctx context.Context, req *multinodepb.TrustedSatellitesRequest) (_ *multinodepb.TrustedSatellitesResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, node.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	response := new(multinodepb.TrustedSatellitesResponse)

	satellites := node.trust.GetSatellites(ctx)
	for _, satellite := range satellites {
		nodeURL, err := node.trust.GetNodeURL(ctx, satellite)
		if err != nil {
			return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
		}

		response.TrustedSatellites = append(response.TrustedSatellites, &multinodepb.TrustedSatellitesResponse_NodeURL{
			NodeId:  nodeURL.ID,
			Address: nodeURL.Address,
		})
	}

	return response, nil
}

// Operator returns operators data.
func (node *NodeEndpoint) Operator(ctx context.Context, req *multinodepb.OperatorRequest) (_ *multinodepb.OperatorResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	if err = authenticate(ctx, node.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}
	return &multinodepb.OperatorResponse{
		Email:          node.config.Email,
		Wallet:         node.config.Wallet,
		WalletFeatures: node.config.WalletFeatures,
	}, nil
}
