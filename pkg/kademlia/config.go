// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

var (
	// Error defines a Kademlia error
	Error = errs.Class("kademlia error")
	mon   = monkit.Package()
)

//CtxKey Used as kademlia key
type CtxKey int

const (
	ctxKeyKad CtxKey = iota
)

// Config defines all of the things that are needed to start up Kademlia
// server endpoints (and not necessarily client code).
type Config struct {
	BootstrapAddr string `help:"the kademlia node to bootstrap against" default:"bootstrap-dev.storj.io:8080"`
	// TODO(jt): remove this! kademlia should just use the grpc server
	TODOListenAddr string `help:"the host/port for kademlia to listen on. TODO(jt): this should be removed!" default:"127.0.0.1:7776"`
}

// Run implements provider.Responsibility
func (c Config) Run(ctx context.Context, server *provider.Provider) (
	err error) {

	defer mon.Task()(&ctx)(&err)

	// TODO(coyle): I'm thinking we just remove  this function and grab from the config.
	in, err := GetIntroNode(c.BootstrapAddr)
	if err != nil {
		return err
	}

	// TODO(jt): kademlia should register on server.GRPC() instead of listening
	// itself
	in.Id = "foo"
	kad, err := NewKademlia(server.Identity().ID, []pb.Node{*in}, c.TODOListenAddr, server.Identity())
	if err != nil {
		return err
	}
	defer func() { _ = kad.Disconnect() }()

	mn := node.NewServer(kad)
	pb.RegisterNodesServer(server.GRPC(), mn)

	// TODO(jt): Bootstrap should probably be blocking and we should kick it off
	// in a goroutine here
	if err = kad.Bootstrap(ctx); err != nil {
		return err
	}

	return server.Run(context.WithValue(ctx, ctxKeyKad, kad))
}

// LoadFromContext loads an existing Kademlia from the Provider context
// stack if one exists.
func LoadFromContext(ctx context.Context) *Kademlia {
	if v, ok := ctx.Value(ctxKeyKad).(*Kademlia); ok {
		return v
	}
	return nil
}
