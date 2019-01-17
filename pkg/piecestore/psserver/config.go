// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/context"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/psserver/agreementsender"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/provider"
)

var (
	mon = monkit.Package()
)

// Config contains everything necessary for a server
type Config struct {
	Path                         string        `help:"path to store data in" default:"$CONFDIR/storage"`
	AllocatedDiskSpace           memory.Size   `user:"true" help:"total allocated disk space in bytes" default:"1TiB"`
	AllocatedBandwidth           memory.Size   `user:"true" help:"total allocated bandwidth in bytes" default:"500GiB"`
	KBucketRefreshInterval       time.Duration `help:"how frequently Kademlia bucket should be refreshed with node stats" default:"1h0m0s"`
	AgreementSenderCheckInterval time.Duration `help:"duration between agreement checks" default:"1h0m0s"`
}

// Run implements provider.Responsibility
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	// piecestore Storage Driver
	storage := pstore.NewStorage(filepath.Join(c.Path, "piece-store-data"))

	db, err := psdb.Open(ctx, storage, filepath.Join(c.Path, "piecestore.db"))
	if err != nil {
		return ServerError.Wrap(err)
	}

	// Load kademlia from context
	kad := kademlia.LoadFromContext(ctx)
	if kad == nil {
		return ServerError.New("Failed to load Kademlia from context")
	}

	// Initialize piecestore server struct
	s, err := NewEndpoint(zap.L(), c, storage, db, server.Identity().Key, kad)
	if err != nil {
		return err
	}
	defer func() { log.Fatal(s.Stop(ctx)) }()

	pb.RegisterPieceStoreRoutesServer(server.GRPC(), s)

	rt, err := kad.GetRoutingTable(ctx)
	if err != nil {
		return ServerError.Wrap(err)
	}

	krt, ok := rt.(*kademlia.RoutingTable)
	if !ok {
		return ServerError.New("Could not convert dht.RoutingTable to *kademlia.RoutingTable")
	}

	// Initialize Refresh process for updating storage node meta in kademlia
	refreshProcess := newService(zap.L(), c.KBucketRefreshInterval, krt, s)
	go refreshProcess.Run(ctx)

	// Initialize agreementsender process for sending received bandwidth agreements to satellites
	agreementSender := agreementsender.New(zap.L(), s.DB, server.Identity(), kad, c.AgreementSenderCheckInterval)
	go agreementSender.Run(ctx)

	go func() {
		done := ctx.Done()
		<-done
		if err := server.Close(); err != nil {
			zap.L().Error("closing server", zap.Error(err))
		}
	}()

	s.log.Info("Started Node", zap.String("ID", fmt.Sprint(server.Identity().ID)))
	return server.Run(ctx)
}
