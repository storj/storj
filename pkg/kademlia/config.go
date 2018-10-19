// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"

	"storj.io/storj/pkg/utils"

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
	DBPath        string `help:"the path for our db services to be created on" default:"$CONFDIR/kademlia"`
	// TODO(jt): remove this! kademlia should just use the grpc server
	TODOListenAddr              string `help:"the host/port for kademlia to listen on. TODO(jt): this should be removed!" default:"127.0.0.1:7776"`
	Alpha                       int    `help:"alpha is a system wide concurrency parameter." default:"5"`
	DefaultIDLength             int    `help:"Length of Kademlia Node ID's. This is tied to provider.FullIdentity." default:"256"`
	DefaultBucketSize           int    `help:"Size of each Kademlia bucket." default:"20"`
	DefaultReplacementCacheSize int    `help:"Size of Replacement Cache" default:"5"`
}

// KadConfig defines the parameters for Kademlia to operate and
// exposes them to the Config struct for easier use.
type KadConfig struct {
	Alpha                       int
	DefaultIDLength             int
	DefaultBucketSize           int
	DefaultReplacementCacheSize int
}

// Run implements provider.Responsibility
func (c Config) Run(ctx context.Context, server *provider.Provider) (
	err error) {

	defer mon.Task()(&ctx)(&err)

	// Create a KadConfig from the root Config
	kadconfig := KadConfig{
		Alpha:                       c.Alpha,
		DefaultIDLength:             c.DefaultIDLength,
		DefaultBucketSize:           c.DefaultBucketSize,
		DefaultReplacementCacheSize: c.DefaultReplacementCacheSize,
	}

	// TODO(coyle): I'm thinking we just remove  this function and grab from the config.
	in, err := GetIntroNode(c.BootstrapAddr)
	if err != nil {
		return err
	}

	// TODO(jt): kademlia should register on server.GRPC() instead of listening
	// itself
	in.Id = "foo"
	kad, err := NewKademlia(server.Identity().ID, []pb.Node{*in}, c.TODOListenAddr, server.Identity(), c.DBPath, kadconfig)
	if err != nil {
		return err
	}
	defer func() {
		rerr := kad.Disconnect(ctx)
		if rerr != nil {
			err = utils.CombineErrors(err, rerr)
		}
	}()

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
