// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psservice

import (
	"context"
	"log"
	"path/filepath"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	psserver "storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/piecestore/rpc/server/ttl"
	"storj.io/storj/pkg/provider"
	pspb "storj.io/storj/protos/piecestore"
)

var (
	// Error represents a farmer error
	Error = errs.Class("farmer error")
	mon   = monkit.Package()
)

// Config is a configuration struct that implements all the configuration
// needed for the piece store responsibility
type Config struct {
	Path string `help:"path to store data in" default:"$CONFDIR"`
}

// Run implements provider.Responsibility
func (c Config) Run(ctx context.Context, server *provider.Provider) (
	err error) {
	defer mon.Task()(&ctx)(&err)

	ttlPath := filepath.Join(c.Path, "ttl.db")
	piecePath := filepath.Join(c.Path, "data")

	ttldb, err := ttl.NewTTL(ttlPath)
	if err != nil {
		return err
	}
	// TODO(jt): defer ttldb.Close()

	// TODO(jt): server.Server constructor
	s := &psserver.Server{PieceStoreDir: piecePath, DB: ttldb}
	// TODO(jt): defer s.Close()

	pspb.RegisterPieceStoreRoutesServer(server.GRPC(), s)

	go func() {
		// TODO(jt): why isn't the piecestore server doing this?
		log.Fatal(s.DB.DBCleanup(piecePath))
	}()

	return server.Run(ctx)
}
