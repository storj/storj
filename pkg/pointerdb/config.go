// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/storelogger"
)

const (
	// PointerBucket is the string representing the bucket used for `PointerEntries`
	PointerBucket = "pointers"
)

// Config is a configuration struct that is everything you need to start a
// PointerDB responsibility
type Config struct {
	DatabaseURL          string `help:"the database connection string to use" default:"bolt://$CONFDIR/pointerdb.db"`
	MinRemoteSegmentSize int    `default:"1240" help:"minimum remote segment size"`
	MaxInlineSegmentSize int    `default:"8000" help:"maximum inline segment size"`
	Overlay              bool   `default:"false" help:"toggle flag if overlay is enabled"`
}

// Run implements the provider.Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) error {
	dburl, err := utils.ParseURL(c.DatabaseURL)
	if err != nil {
		return err
	}
	if dburl.Scheme != "bolt" {
		return Error.New("unsupported db scheme: %s", dburl.Scheme)
	}

	bdb, err := boltdb.New(dburl.Path, PointerBucket)
	if err != nil {
		return err
	}
	defer func() { _ = bdb.Close() }()

	cache := overlay.LoadFromContext(ctx)
	bdblogged := storelogger.New(zap.L(), bdb)
	pb.RegisterPointerDBServer(server.GRPC(), NewServer(bdblogged, cache, zap.L(), c, server.Identity()))

	return server.Run(ctx)
}
