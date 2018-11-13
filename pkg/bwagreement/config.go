// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	dbx "storj.io/storj/pkg/bwagreement/dbx"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

var (
	mon = monkit.Package()
)

// Config is a configuration struct that is everything you need to start an
// agreement receiver responsibility
type Config struct {
	DatabaseURL    string `help:"the database connection string to use" default:"postgres://postgres@localhost/pointerdb?sslmode=disable"`
	DatabaseDriver string `help:"the database driver to use" default:"postgres"`
}

// Run implements the provider.Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)
	var db *dbx.DB
	// Attempt to connect to database a few times, backing off more each time
	// there's a failure
	for backoff := time.Second; backoff < 60*time.Second; backoff *= 2 {
		db, err = dbx.Open(c.DatabaseDriver, c.DatabaseURL)
		if err != nil {
			zap.L().Warn("Error connecting to bwagreement database.",
				zap.Error(err),
				zap.Duration("backoff", backoff),
			)
			select {
			case <-time.After(backoff):
				continue
			case <-ctx.Done():
				return fmt.Errorf("Cancelled before backoff time expired")
			}
		}
	}

	// If there's an error connecting to the database, fail here and return the
	// error to the caller.
	if err != nil {
		return fmt.Errorf("Can't connect to the database for bwagreement: %s", err)
	}

	zap.L().Info("Connected to bwagreement database")

	k := server.Identity().Leaf.PublicKey
	ns, err := NewServer(db, zap.L(), k)
	if err != nil {
		return fmt.Errorf("Can't connect to the database for bwagreement: %s", err)
	}

	pb.RegisterBandwidthServer(server.GRPC(), ns)

	return server.Run(ctx)
}
