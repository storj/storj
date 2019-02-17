// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
)

var (
	mon = monkit.Package()
)

// Endpoint implements the network state RPC service
type Endpoint struct {
	log         *zap.Logger
	config      Config
	pdbEndpoint *pointerdb.Server
	ocEndpoint  *overlay.Server
}

// NewEndpoint creates instance of Endpoint
func NewEndpoint(log *zap.Logger, config Config, pdbEndpoint *pointerdb.Server, ocEndpoint *overlay.Server) *Endpoint {
	return &Endpoint{
		log:         log,
		config:      config,
		pdbEndpoint: pdbEndpoint,
		ocEndpoint:  ocEndpoint,
	}
}

// Close closes resources
func (e *Endpoint) Close() error { return nil }

// Health returns the health of a specific path
func (e *Endpoint) Health(ctx context.Context, req *pb.ObjectHealthRequest) (resp *pb.ObjectHealthResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	resp = &pb.ObjectHealthResponse{}

	// get the stream's info thru last segment ie l/<path>
	pdbResp, err := e.pdbEndpoint.Get(ctx, &pb.GetRequest{Path: req.GetPath()})

	// for each segment
	// 		determine number of good nodes and bad nodes
	// 		append to Response

	return resp, nil
}
