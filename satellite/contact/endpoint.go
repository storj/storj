// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"

	"go.uber.org/zap"
)

// Endpoint implements the contact service Endpoints.
type Endpoint struct {
	log     *zap.Logger
	service *Service
}

// NewEndpoint returns a new contact service endpoint.
func NewEndpoint(log *zap.Logger, service *Service) *Endpoint {
	return &Endpoint{
		log:     log,
		service: service,
	}
}

// These are being created in another branch; these dummy types should be removed once we can
// switch to real protobuf types.
type dummyCheckinRequest struct{}
type dummyCheckinResponse struct{}

// Checkin is periodically called by storage nodes to keep the satellite informed of its existence,
// address, and operator information. In return, this satellite keeps the node informed of its
// reachability.
func (endpoint *Endpoint) Checkin(ctx context.Context, req *dummyCheckinRequest) (_ *dummyCheckinResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: record information, pingback node here

	return &dummyCheckinResponse{}, nil
}
