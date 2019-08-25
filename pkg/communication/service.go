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

var (
	// NodeErr is the class for all errors pertaining to node operations
	NodeErr = errs.Class("node error")
)

type Communication struct {
	log        *zap.Logger
	self       *overlay.NodeDossier
	dialer     *Dialer
	mu         sync.Mutex
	lastPinged time.Time
}

func NewService(log *zap.Logger, config Config, self *overlay.NodeDossier, transport transport.Client) *Communication {
	return &Communication{
		log:    log,
		self:   self,
		dialer: NewDialer(log.Named("dialer"), config, transport),
	}
}

// LastPinged returns last time someone pinged this node.
func (c *Communication) LastPinged() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastPinged
}

// Pinged notifies the service it has been remotely pinged.
func (c *Communication) Pinged() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPinged = time.Now()
}

// Local returns the local node
func (c *Communication) Local() overlay.NodeDossier {
	return *c.self
}

// Ping checks that the provided node is still accessible on the network
func (c *Communication) Ping(ctx context.Context, node pb.Node) (_ pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	ok, err := c.dialer.PingNode(ctx, node)
	if err != nil {
		return pb.Node{}, NodeErr.Wrap(err)
	}
	if !ok {
		return pb.Node{}, NodeErr.New("%s : %s failed to ping node ID %s", c.self.Type.String(), c.self.Id.String(), node.Id.String())
	}
	return node, nil
}

// FetchInfo connects to a node address and returns the node info
func (c *Communication) FetchInfo(ctx context.Context, node pb.Node) (_ *pb.InfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	info, err := c.dialer.FetchInfo(ctx, node)
	if err != nil {
		return nil, NodeErr.Wrap(err)
	}
	return info, nil
}
