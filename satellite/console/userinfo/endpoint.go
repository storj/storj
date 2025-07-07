// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package userinfo

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/satellite/console"
)

var (
	mon = monkit.Package()
	// Error is an error class for userinfo endpoint errors.
	Error = errs.Class("userinfo_endpoint")
)

// Config holds Endpoint's configuration.
type Config struct {
	Enabled      bool           `help:"Whether the private Userinfo rpc endpoint is enabled" default:"false"`
	AllowedPeers storj.NodeURLs `help:"A comma delimited list of peers (IDs/addresses) allowed to use this endpoint."`
}

// Endpoint userinfo endpoint.
type Endpoint struct {
	pb.DRPCUserInfoUnimplementedServer

	log          *zap.Logger
	users        console.Users
	apiKeys      console.APIKeys
	projects     console.Projects
	config       Config
	allowedPeers map[storj.NodeID]storj.NodeURL
}

// NewEndpoint creates a new userinfo endpoint instance.
func NewEndpoint(log *zap.Logger, users console.Users, apiKeys console.APIKeys, projects console.Projects, config Config) (*Endpoint, error) {
	if len(config.AllowedPeers) == 0 {
		return nil, Error.New("allowed peer list parameter '--allowed-peer-list' is required")
	}

	// put peers into a map for faster retrieval by NodeID.
	allowedPeers := make(map[storj.NodeID]storj.NodeURL)
	for _, peer := range config.AllowedPeers {
		allowedPeers[peer.ID] = peer
	}

	return &Endpoint{
		log:          log,
		users:        users,
		apiKeys:      apiKeys,
		projects:     projects,
		config:       config,
		allowedPeers: allowedPeers,
	}, nil
}

// Close closes resources.
func (e *Endpoint) Close() error { return nil }

// Get returns relevant info about the current user.
func (e *Endpoint) Get(ctx context.Context, req *pb.GetUserInfoRequest) (response *pb.GetUserInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := rpcpeer.FromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	peerID, err := identity.PeerIdentityFromPeer(peer)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}

	if err = e.verifyPeer(peerID.ID); err != nil {
		return nil, rpcstatus.Error(rpcstatus.PermissionDenied, err.Error())
	}

	key, err := e.getAPIKey(ctx, req.Header)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
	}

	info, err := e.apiKeys.GetByHead(ctx, key.Head())
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, Error.Wrap(err).Error())
	}

	project, err := e.projects.Get(ctx, info.ProjectID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.NotFound, Error.Wrap(err).Error())
	}

	user, err := e.users.Get(ctx, project.OwnerID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.NotFound, Error.Wrap(err).Error())
	}

	return &pb.GetUserInfoResponse{
		PaidTier: user.HasPaidPrivileges(),
	}, nil
}

// verifyPeer verifies that a peer is allowed.
func (e *Endpoint) verifyPeer(id storj.NodeID) error {
	_, ok := e.allowedPeers[id]
	if !ok {
		return Error.New("peer %q is untrusted", id)
	}
	return nil
}

func (e *Endpoint) getAPIKey(ctx context.Context, header *pb.RequestHeader) (key *macaroon.APIKey, err error) {
	defer mon.Task()(&ctx)(&err)

	if header == nil {
		return nil, Error.New("Missing API credentials")
	}

	key, err = macaroon.ParseRawAPIKey(header.ApiKey)
	if err != nil {
		err = Error.Wrap(err)
		e.log.Debug("Invalid credentials", zap.Error(err))
		return nil, err
	}

	return key, nil
}
