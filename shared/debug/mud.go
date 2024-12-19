// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package debug

import (
	"context"
	"net"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/debug"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/mud"
)

// Wrapper combines the debug server and the listener.
type Wrapper struct {
	Listener net.Listener
	Server   *debug.Server
}

// NewWrapper creates a new debug server and listener.
func NewWrapper(logger *zap.Logger, config debug.Config, extensions []debug.Extension) (Wrapper, error) {
	var d Wrapper
	var err error

	if config.Addr == "" {
		return d, nil
	}

	namedLog := logger.Named("debug")

	d.Listener, err = net.Listen("tcp", config.Addr)
	if err != nil {
		withoutStack := errs.New("%s", err.Error())
		logger.Debug("failed to start debug endpoints", zap.Error(withoutStack))
	} else {
		namedLog.Info("debug server is started listening", zap.Stringer("addr", d.Listener.Addr()))
	}

	d.Server = debug.NewServer(namedLog, d.Listener, monkit.Default, config, extensions...)
	return d, nil
}

// Run starts the debug server.
func (d Wrapper) Run(ctx context.Context) error {
	if d.Server != nil {
		return d.Server.Run(ctx)
	}
	return nil
}

// Close stops the debug server.
func (d Wrapper) Close(ctx context.Context) error {
	if d.Server == nil {
		return nil
	}
	return d.Server.Close()
}

// Module is a mud module definition.
func Module(ball *mud.Ball) {
	mud.Provide[Wrapper](ball, NewWrapper)
	mud.Provide[*ModuleGraph](ball, func(ball *mud.Ball) *ModuleGraph {
		return &ModuleGraph{
			ball: ball,
		}
	})
	mud.Implementation[[]debug.Extension, *ModuleGraph](ball)
	mud.RemoveTag[*ModuleGraph, mud.Optional](ball)
	mud.Tag[Wrapper, modular.Service](ball, modular.Service{})
}
