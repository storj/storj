// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package overlay

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/statdb"
	statpb "storj.io/storj/pkg/statdb/proto"
)

func TestRun(t *testing.T) {
	bctx := context.Background()
	ctxWithAPIKey := auth.WithAPIKey(bctx, []byte(""))

	kad := &kademlia.Kademlia{}
	var kadKey kademlia.CtxKey
	ctxWithKad := context.WithValue(ctxWithAPIKey, kadKey, kad)

	prv, address, err := getProvider(ctxWithKad)
	assert.NoError(t, err)
	assert.NotNil(t, prv)

	// run with nil
	err = Config{}.Run(context.Background(), prv)
	assert.Error(t, err)
	assert.Equal(t, "overlay error: programmer error: kademlia responsibility unstarted", err.Error())

	// run with nil, pass pointer to Kademlia in context
	err = Config{StatDBPort: address}.Run(ctxWithKad, prv)
	assert.Error(t, err)
	assert.Equal(t, "overlay error: database scheme not supported: ", err.Error())

	// db scheme redis conn fail
	err = Config{DatabaseURL: "redis://somedir/overlay.db/?db=1", StatDBPort: address}.Run(ctxWithKad, prv)

	assert.Error(t, err)
	assert.Equal(t, "redis error: ping failed: dial tcp: address somedir: missing port in address", err.Error())

	// db scheme bolt conn fail
	err = Config{DatabaseURL: "bolt://somedir/overlay.db", StatDBPort: address}.Run(ctxWithKad, prv)
	assert.Error(t, err)
}

func registerStatDBServer(srv *grpc.Server) (err error) {
	dbPath := fmt.Sprintf("file:memdb%d?mode=memory&cache=shared", rand.Int63())
	sdb, err := statdb.NewServer("sqlite3", dbPath, zap.NewNop())
	if err != nil {
		return err
	}
	statpb.RegisterStatDBServer(srv, sdb)
	return nil
}

func getProvider(ctx context.Context) (*provider.Provider, string, error) {
	ca, err := provider.NewTestCA(ctx)
	if err != nil {
		return nil, "", err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, "", err
	}
	identOpt, err := identity.ServerOption()
	if err != nil {
		return nil, "", err
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	srv := grpc.NewServer(identOpt)
	err = registerStatDBServer(srv)
	if err != nil {
		return nil, "", err
	}
	go func() {
		_ = srv.Serve(lis)
	}()
	defer func() {
		_ = lis.Close()
	}()
	address := lis.Addr().String()

	prv, err := provider.NewProvider(identity, lis, nil)
	if err != nil {
		return nil, "", err
	}
	return prv, address, nil
}
