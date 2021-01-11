// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
	"storj.io/private/version"
	"storj.io/storj/private/multinodepb"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/reputation"
)

var _ multinodepb.DRPCNodeServer = (*NodeEndpoint)(nil)

// NodeEndpoint implements multinode node endpoint.
//
// architecture: Endpoint
type NodeEndpoint struct {
	log        *zap.Logger
	apiKeys    *apikeys.Service
	version    version.Info
	contact    *contact.PingStats
	reputation reputation.DB
}

// NewNodeEndpoint creates new multinode node endpoint.
func NewNodeEndpoint(log *zap.Logger, apiKeys *apikeys.Service, version version.Info, contact *contact.PingStats, reputation reputation.DB) *NodeEndpoint {
	return &NodeEndpoint{
		log:        log,
		apiKeys:    apiKeys,
		version:    version,
		contact:    contact,
		reputation: reputation,
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

	return &multinodepb.ReputationResponse{
		Online: &multinodepb.ReputationResponse_Online{
			Score: rep.OnlineScore,
		},
		Audit: &multinodepb.ReputationResponse_Audit{
			Score:           rep.Audit.Score,
			SuspensionScore: rep.Audit.UnknownScore,
		},
	}, nil
}
