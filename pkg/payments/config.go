// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

var (
	mon = monkit.Package()
	// Error is the main payments error class for this package
	Error = errs.Class("payments server error: ")
)

// Config is a configuration struct for everything you need to start the
// Payments responsibility.
type Config struct {
	//Filepath
	Filepath string `help:"the file path of the generated csv" default:"$CONFDIR/payments"`
}

// Run implements the provider.Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)
	db, ok := ctx.Value("masterdb").(interface {
		Accounting() accounting.DB
		OverlayCache() overlay.DB
	})
	if !ok {
		return Error.New("unable to get master db instance")
	}
	srv := &Server{
		filepath:     c.Filepath,
		accountingDB: db.Accounting(),
		overlayDB:    db.OverlayCache(),
		log:          zap.L(),
		metrics:      monkit.Default,
	}

	pb.RegisterPaymentsServer(server.GRPC(), srv)

	return server.Run(ctx)
}
