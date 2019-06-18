// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var (
	// Error defines a Kademlia error
	Error = errs.Class("kademlia error")
	// mon   = monkit.Package() // TODO: figure out whether this is needed
)

// Config defines all of the things that are needed to start up Kademlia
// server endpoints (and not necessarily client code).
type Config struct {
	BootstrapAddr        string        `help:"the Kademlia nodes to bootstrap against" releaseDefault:"bootstrap.storj.io:8888" devDefault:""`
	BootstrapBackoffMax  time.Duration `help:"the maximum amount of time to wait when retrying bootstrap" default:"30s"`
	BootstrapBackoffBase time.Duration `help:"the base interval to wait when retrying bootstrap" default:"1s"`
	DBPath               string        `help:"the path for storage node db services to be created on" default:"$CONFDIR/kademlia"`
	ExternalAddress      string        `user:"true" help:"the public address of the Kademlia node, useful for nodes behind NAT" default:""`
	Operator             OperatorConfig

	// TODO: reduce the number of flags here
	Alpha int `help:"alpha is a system wide concurrency parameter" default:"5"`
	RoutingTableConfig
}

// BootstrapNodes returns bootstrap nodes defined in the config
func (c Config) BootstrapNodes() ([]*pb.Node, error) {
	return NodesFromConfig(c.BootstrapAddr)
}

// NodesFromConfig parses a comma-separated list of nodes and returns []pb.Node.
// A node requires an ipv4/ipv6 host/port pair, and optionally supports a
// node id annotation.
// The below is the parse grammar in BNF:
//
//   cfg  ::= <node> (`,` <node>)*
//	 node ::= ( <ipv4> | `[` <ipv6> `]` ) `:` <port> [ `#` <nodeid> ]
//
// Examples of individual nodes:
//
//   33.20.0.1:7777
//   33.20.0.1:7777#ekC4dHif4NAGTTFtniBbcLuhPGoujdgNIJf313
//	 [2001:db8:1f70::999:de8:7648:6e8]:7777
//	 [2001:db8:1f70::999:de8:7648:6e8]:7777#ekC4dHif4NAGTTFtniBbcLuhPGoujdgNIJf313
//
func NodesFromConfig(cfg string) (nodes []*pb.Node, err error) {
	parts := strings.Split(cfg, ",")
	nodes = make([]*pb.Node, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) == 0 {
			continue
		}
		node, err := parseNode(part)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &node)
	}
	return nodes, nil
}

func parseNode(node string) (rv pb.Node, err error) {
	parts := strings.Split(node, "#")
	if len(parts) > 2 {
		return pb.Node{}, Error.New("invalid node spec: %q", node)
	}
	// just check that this is sufficiently well-formed
	_, _, err = net.SplitHostPort(parts[0])
	if err != nil {
		return pb.Node{}, Error.Wrap(err)
	}
	rv = pb.Node{
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   parts[0],
		},
	}
	if len(parts) > 1 {
		rv.Id, err = storj.NodeIDFromString(parts[1])
		if err != nil {
			return pb.Node{}, err
		}
	}
	return rv, nil
}

// Verify verifies whether kademlia config is valid.
func (c Config) Verify(log *zap.Logger) error {
	return c.Operator.Verify(log)
}

// OperatorConfig defines properties related to storage node operator metadata
type OperatorConfig struct {
	Email  string `user:"true" help:"operator email address" default:""`
	Wallet string `user:"true" help:"operator wallet address" default:""`
}

// Verify verifies whether operator config is valid.
func (c OperatorConfig) Verify(log *zap.Logger) error {
	if err := isOperatorEmailValid(log, c.Email); err != nil {
		return err
	}
	if err := isOperatorWalletValid(log, c.Wallet); err != nil {
		return err
	}
	return nil
}

func isOperatorEmailValid(log *zap.Logger, email string) error {
	if email == "" {
		log.Sugar().Warn("Operator email address isn't specified.")
	} else {
		log.Sugar().Info("Operator email: ", email)
	}
	return nil
}

func isOperatorWalletValid(log *zap.Logger, wallet string) error {
	if wallet == "" {
		return fmt.Errorf("Operator wallet address isn't specified")
	}
	r := regexp.MustCompile("^0x[a-fA-F0-9]{40}$")
	if match := r.MatchString(wallet); !match {
		return fmt.Errorf("Operator wallet address isn't valid")
	}

	log.Sugar().Info("Operator wallet: ", wallet)
	return nil
}
