// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"
	"net/url"

	"go.uber.org/zap"

	"storj.io/storj/pkg/provider"
	proto "storj.io/storj/protos/pointerdb"
	"storj.io/storj/storage/boltdb"
)

// Config is a configuration struct that is everything you need to start a
// PointerDB responsibility
type Config struct {
	DatabaseURL string `help:"the database connection string to use" default:"bolt://$CONFDIR/pointerdb.db"`
}

// Run implements the provider.Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) error {
	dburl, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return err
	}
	if dburl.Scheme != "bolt" {
		return Error.New("unsupported db scheme: %s", dburl.Scheme)
	}
	bdb, err := boltdb.NewClient(zap.L(), dburl.Path, boltdb.PointerBucket)
	if err != nil {
		return err
	}
	defer func() { _ = bdb.Close() }()

	proto.RegisterPointerDBServer(server.GRPC(), NewServer(bdb, zap.L()))

	return server.Run(ctx)
}
