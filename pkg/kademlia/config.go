// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"fmt"
	"regexp"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

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
	ExternalAddress      string        `user:"true" help:"the public address of the Kademlia node, useful for nodes behind NAT" default:""`
	Operator             OperatorConfig

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
		return fmt.Errorf("operator wallet address isn't specified")
	}
	r := regexp.MustCompile("^0x[a-fA-F0-9]{40}$")
	if match := r.MatchString(wallet); !match {
		return fmt.Errorf("operator wallet address isn't valid")
	}

	log.Sugar().Info("operator wallet: ", wallet)
	return nil
}
