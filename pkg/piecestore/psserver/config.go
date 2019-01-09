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

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psserver/agreementsender"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/provider"
)

var (
	mon = monkit.Package()
)

// Config contains everything necessary for a server
type Config struct {
	Path                   string        `help:"path to store data in" default:"$CONFDIR"`
	AllocatedDiskSpace     string        `help:"total allocated disk space, default(1TiB)" default:"1TiB"`
	AllocatedBandwidth     string        `help:"total allocated bandwidth, default(500GiB)" default:"500GiB"`
	KBucketRefreshInterval time.Duration `help:"how frequently checker should audit segments" default:"3600s"`
}

// Run implements provider.Responsibility
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)
	ctx, cancel := context.WithCancel(ctx)

	//piecestore
	db, err := psdb.Open(ctx, filepath.Join(c.Path, "piece-store-data"), filepath.Join(c.Path, "piecestore.db"))
	if err != nil {
		return ServerError.Wrap(err)
	}
	s, err := NewEndpoint(zap.L(), c, db, server.Identity().Key)
	if err != nil {
		return err
	}
	pb.RegisterPieceStoreRoutesServer(server.GRPC(), s)

	//kademlia
	k := kademlia.LoadFromContext(ctx)
	if k == nil {
		return ServerError.New("Failed to load Kademlia from context")
	}
	rt, err := k.GetRoutingTable(ctx)
	if err != nil {
		return ServerError.Wrap(err)
	}
	krt, ok := rt.(*kademlia.RoutingTable)
	if !ok {
		return ServerError.New("Could not convert dht.RoutingTable to *kademlia.RoutingTable")
	}
	refreshProcess := newService(zap.L(), c.KBucketRefreshInterval, krt, s)
	go func() {
		if err := refreshProcess.Run(ctx); err != nil {
			cancel()
		}
	}()

	//agreementsender
	agreementsender := agreementsender.New(zap.L(), s.DB, server.Identity(), k)
	go agreementsender.Run(ctx)

	defer func() { log.Fatal(s.Stop(ctx)) }()
	s.log.Info("Started Node", zap.String("ID", fmt.Sprint(server.Identity().ID)))
	return server.Run(ctx)
}
