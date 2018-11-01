// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	//"fmt"
	//"math/rand"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
	//"storj.io/storj/pkg/statdb"
	//statpb "storj.io/storj/pkg/statdb/proto"
	"storj.io/storj/storage/teststore"
)

// NewMockServer provides a mock grpc server for testing
func NewMockServer(items []storage.ListItem, opts ...grpc.ServerOption) *grpc.Server {
	grpcServer := grpc.NewServer(opts...)

	registry := monkit.Default

	k := kademlia.NewMockKademlia()

	c := &Cache{
		DB:  teststore.New(),
		DHT: k,
	}

	/*sdbPath := fmt.Sprintf("file:memdb%d?mode=memory&cache=shared", rand.Int63())
	sdb, err := statdb.NewServer("sqlite3", sdbPath, zap.NewNop())
	if err != nil {
		return err
	}

	statpb.RegisterStatDBServer(grpcServer, sdb)*/

	_ = storage.PutAll(c.DB, items...)

	s := Server{
		dht:     k,
		cache:   c,
		//sdb:  sdb,
		logger:  zap.NewNop(),
		metrics: registry,
	}
	pb.RegisterOverlayServer(grpcServer, &s)

	return grpcServer
}
