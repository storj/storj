// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package communication

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/overlay"
)

type Config struct {
	ExternalAddress string `user:"true" help:"the public address of the node, useful for nodes behind NAT" default:""`
	DialerLimit     int    `help:"Semaphore size" Default:"32"`
}

var (
	// NodeErr is the class for all errors pertaining to node operations
	NodeErr = errs.Class("node error")
)

// Service is the communication service between storage nodes and satellites
type Service struct {
	log        *zap.Logger
	self       *overlay.NodeDossier
	dialer     *Dialer
	mu         sync.Mutex
	lastPinged time.Time
}

// NewService creates a new communication service
func NewService(log *zap.Logger, config Config, self *overlay.NodeDossier, transport transport.Client) *Service {
	return &Service{
		log:    log,
		self:   self,
		dialer: NewDialer(log.Named("dialer"), config, transport),
	}
}

// LastPinged returns last time someone pinged this node.
func (service *Service) LastPinged() time.Time {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.lastPinged
}

// Pinged notifies the service it has been remotely pinged.
func (service *Service) Pinged() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.lastPinged = time.Now()
}

// Local returns the local node
func (service *Service) Local() overlay.NodeDossier {
	return *service.self
}

// Ping checks that the provided node is still accessible on the network
func (service *Service) Ping(ctx context.Context, node pb.Node) (_ pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	ok, err := service.dialer.PingNode(ctx, node)
	if err != nil {
		return pb.Node{}, NodeErr.Wrap(err)
	}
	if !ok {
		return pb.Node{}, NodeErr.New("%s : %s failed to ping node ID %s", service.self.Type.String(), service.self.Id.String(), node.Id.String())
	}
	return node, nil
}

// FetchInfo connects to a node address and returns the node info
func (service *Service) FetchInfo(ctx context.Context, node pb.Node) (_ *pb.InfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	info, err := service.dialer.FetchInfo(ctx, node)
	if err != nil {
		return nil, NodeErr.Wrap(err)
	}
	return info, nil
}

// UpdateSelf updates the local node with the provided info
func (service *Service) UpdateSelf(capacity *pb.NodeCapacity) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if capacity != nil {
		service.self.Capacity = *capacity
	}
}
