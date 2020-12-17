// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodes

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
)

var (
	mon = monkit.Package()

	// Error is an error class for nodes service error.
	Error = errs.Class("nodes service error")
)

// Service exposes all nodes related logic.
//
// architecture: Service
type Service struct {
	log   *zap.Logger
	nodes DB
}

// NewService creates new instance of Service.
func NewService(log *zap.Logger, nodes DB) *Service {
	return &Service{
		log:   log,
		nodes: nodes,
	}
}

// Add adds new node to the system.
func (service *Service) Add(ctx context.Context, id storj.NodeID, apiSecret []byte, publicAddress string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.Wrap(service.nodes.Add(ctx, id, apiSecret, publicAddress))
}

// UpdateName will update name of the specified node.
func (service *Service) UpdateName(ctx context.Context, id storj.NodeID, name string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.Wrap(service.nodes.UpdateName(ctx, id, name))
}

// Get retrieves node by id.
func (service *Service) Get(ctx context.Context, id storj.NodeID) (_ Node, err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.nodes.Get(ctx, id)
	if err != nil {
		return Node{}, Error.Wrap(err)
	}

	return node, nil

}

// List retrieves list of all added nodes.
func (service *Service) List(ctx context.Context) (_ []Node, err error) {
	defer mon.Task()(&ctx)(&err)

	nodes, err := service.nodes.List(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return nodes, nil
}

// Remove removes node from the system.
func (service *Service) Remove(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.Wrap(service.nodes.Remove(ctx, id))
}
