// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
)

var (
	// Error defines a Kademlia error
	Error = errs.Class("kademlia error")
	// mon   = monkit.Package() // TODO: figure out whether this is needed
)

// Config defines all of the things that are needed to start up Kademlia
// server endpoints (and not necessarily client code).
type Config struct {
	BootstrapAddr        string        `help:"the Kademlia node to bootstrap against" releaseDefault:"bootstrap.storj.io:8888" devDefault:""`
	BootstrapBackoffMax  time.Duration `help:"the maximum amount of time to wait when retrying bootstrap" default:"30s"`
	BootstrapBackoffBase time.Duration `help:"the base interval to wait when retrying bootstrap" default:"1s"`
	DBPath               string        `help:"the path for storage node db services to be created on" default:"$CONFDIR/kademlia"`

	// TODO: reduce the number of flags here
	Alpha int `help:"alpha is a system wide concurrency parameter" default:"5"`
	RoutingTableConfig
}

// BootstrapNodes returns bootstrap nodes defined in the config
func (c Config) BootstrapNodes() []pb.Node {
	var nodes []pb.Node
	if c.BootstrapAddr != "" {
		nodes = append(nodes, pb.Node{
			Address: &pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				Address:   c.BootstrapAddr,
			},
		})
	}
	return nodes
}
