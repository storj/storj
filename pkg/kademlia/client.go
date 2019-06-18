// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

type ClientConfig struct {
	K     int `help:"K is the system-wide Kademlia neighborhood size" default:"20"`
	Alpha int `help:"alpha is a system wide concurrency parameter" default:"5"`
}

type Client struct {
	log    *zap.Logger
	config ClientConfig
	dialer *Dialer

	mu             sync.Mutex
	resolved       bool
	bootstrapNodes []*pb.Node
}

// NewClient returns a newly configured Kademlia client
func NewClient(log *zap.Logger, transport transport.Client, bootstrapNodes []*pb.Node, config ClientConfig) *Client {
	return &Client{
		log:            log,
		config:         config,
		bootstrapNodes: bootstrapNodes,
		dialer:         NewDialer(log.Named("dialer"), transport),
	}
}

func (c *Client) Ping(ctx context.Context, node pb.Node) (err error) {
	defer mon.Task()(&ctx)(&err)
	ok, err := c.dialer.PingNode(ctx, node)
	if err != nil {
		return NodeErr.Wrap(err)
	}
	if !ok {
		return NodeErr.New("Failed pinging node")
	}
	return nil
}

func (c *Client) FetchInfo(ctx context.Context, node pb.Node) (_ *pb.InfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	info, err := c.dialer.FetchInfo(ctx, node)
	return info, NodeErr.Wrap(err)
}

func (c *Client) FindNear(ctx context.Context, target storj.NodeID) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	err = c.resolveBootstrapIds(ctx)
	if err != nil {
		return nil, err
	}
	return newPeerDiscovery(c.log, c.dialer, target, c.bootstrapNodes,
		c.config.K, c.config.Alpha, nil).Run(ctx)
}

func (c *Client) resolveBootstrapIds(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.resolved {
		return nil
	}
	for i, node := range c.bootstrapNodes {
		// TODO: it's not great that the zero value is used to mean no node id.
		// unlikely but all zeros could technically be a valid node id
		if node.Id == (storj.NodeID{}) {
			c.log.Warn("node id missing in bootstrap node config")
			ident, err := c.dialer.FetchPeerIdentityUnverified(ctx, node.Address.Address)
			if err != nil {
				return err
			}
			nodecopy := *node
			nodecopy.Id = ident.ID
			c.bootstrapNodes[i] = &nodecopy
		}
	}
	c.resolved = true
	return nil
}
