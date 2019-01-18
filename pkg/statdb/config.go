// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/server"
)

// Config represents a StatDB service
type Config struct{}

// Run implements server.Service
func (Config) Run(ctx context.Context, server *server.Server) (err error) {
	defer mon.Task()(&ctx)(&err)

	sdb, ok := ctx.Value("masterdb").(interface {
		StatDB() DB
	})
	if !ok {
		return Error.New("unable to get master db instance")
	}

	zap.S().Warn("Once the Peer refactor is done, the statdb inspector needs to be registered on a " +
		"gRPC server that only listens on localhost")
	// TODO: register on a private rpc server
	pb.RegisterStatDBInspectorServer(server.GRPC(), NewInspector(sdb.StatDB()))

	return server.Run(ctx)
}
