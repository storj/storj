// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
)

var (
	mon = monkit.Package()

	// Error is an error class for multinode console service error.
	Error = errs.Class("multinode console service error")
)

// Service encapsulates multinode console logic.
//
// architecture: Service
type Service struct {
	log   *zap.Logger
	nodes Nodes
}

// NewService creates new instance of Service.
func NewService(log *zap.Logger, nodes Nodes) *Service {
	return &Service{
		log:   log,
		nodes: nodes,
	}
}

// AddNode adds new node to the system.
func (service *Service) AddNode(ctx context.Context, id storj.NodeID, apiSecret []byte, publicAddress string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.Wrap(service.nodes.Add(ctx, id, apiSecret, publicAddress))
}

// RemoveNode removes node from the system.
func (service *Service) RemoveNode(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.Wrap(service.nodes.Remove(ctx, id))
}
