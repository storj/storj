// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package crash

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/private/crashreportpb"
)

// ensures that Endpoint implements crashreportpb.DRPCCrashReportServer.
var _ crashreportpb.DRPCCrashReportServer = (*Endpoint)(nil)

// Endpoint is an drpc controller for receiving crashes.
type Endpoint struct {
	crashes *Service
	log     *zap.Logger
}

// NewEndpoint is a constructor for Endpoint.
func NewEndpoint(log *zap.Logger, crashes *Service) *Endpoint {
	return &Endpoint{
		crashes: crashes,
		log:     log,
	}
}

// Report is an drpc endpoint for receiving crashes.
func (endpoint *Endpoint) Report(ctx context.Context, r *crashreportpb.ReportRequest) (*crashreportpb.ReportResponse, error) {
	peerID, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	err = endpoint.crashes.Report(peerID.ID, r.GzippedPanic)
	if err != nil {
		endpoint.log.Error("could not create file with panic", zap.Error(err))

		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return &crashreportpb.ReportResponse{}, nil
}
