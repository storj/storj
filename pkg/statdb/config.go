// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"

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

	pb.RegisterStatDBInspectorServer(server.PrivateRPC(), NewInspector(sdb.StatDB()))

	return server.Run(ctx)
}
