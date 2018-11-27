// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/bwagreement/database-manager"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

var (
	mon = monkit.Package()
)

// Config is a configuration struct that is everything you need to start an
// agreement receiver responsibility
type Config struct {
	DatabaseURL string `help:"the database connection string to use" default:"sqlite3://$CONFDIR/bw.db"`
}

// Run implements the provider.Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)
	k := server.Identity().Leaf.PublicKey

	zap.S().Debug("Starting Bandwidth Agreement Receiver...")

	dbm, err := dbmanager.NewDBManager(c.DatabaseURL)
	if err != nil {
		return errs.New("Error starting initializing database for Bandwidth Agreement server on satellite: %+v", err)
	}

	ns, err := NewServer(dbm, zap.L(), k)
	if err != nil {
		return errs.New("Error starting Bandwidth Agreement server on satellite: %+v", err)
	}

	pb.RegisterBandwidthServer(server.GRPC(), ns)

	return server.Run(ctx)
}
