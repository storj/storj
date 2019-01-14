// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

var (
	mon = monkit.Package()
	// Error is the main payments error class for this package
	Error = errs.Class("metainfo error: ")
)

// Config is a configuration struct for everything you need to start the
// Payments responsibility.
type Config struct{}

// Run implements the provider.Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	db, ok := ctx.Value("masterdb").(interface {
		Metainfo() DB
	})
	if !ok {
		return Error.New("unable to get master db instance")
	}

	srv := &Service{
		db: db.Metainfo(),
	}

	endpoint := &Endpoint{
		service: srv,
	}

	pb.RegisterMetainfoServer(server.GRPC(), endpoint)

	return server.Run(ctx)
}
