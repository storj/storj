// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package overlay

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	"go.uber.org/zap"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/provider"
	statpb "storj.io/storj/pkg/statdb/proto"
)

func TestRun(t *testing.T) {
	config := Config{}
	bctx := context.Background()
	kad := &kademlia.Kademlia{}
	var key kademlia.CtxKey

	prv, address, err := getProvider(bctx)
	assert.NoError(t, err)

	cases := []struct {
		testName string
		testFunc func(t *testing.T)
	}{
		{
			testName: "Run with nil",
			testFunc: func(t *testing.T) {
				err := config.Run(bctx, prv)

				assert.Error(t, err)
				assert.Equal(t, err.Error(), "overlay error: programmer error: kademlia responsibility unstarted")
			},
		},
		{
			testName: "Run with nil, pass pointer to Kademlia in context",
			testFunc: func(t *testing.T) {
				ctx := context.WithValue(bctx, key, kad)
				err := config.Run(ctx, prv)

				assert.Error(t, err)
				assert.Equal(t, err.Error(), "overlay error: database scheme not supported: ")
			},
		},
		{
			testName: "db scheme redis conn fail",
			testFunc: func(t *testing.T) {
				ctx := context.WithValue(bctx, key, kad)
				var config = Config{DatabaseURL: "redis://somedir/overlay.db/?db=1", StatDBPort: address}
				err := config.Run(ctx, prv)

				assert.Error(t, err)
				assert.Equal(t, err.Error(), "redis error: ping failed: dial tcp: address somedir: missing port in address")
			},
		},
		{
			testName: "db scheme bolt conn fail",
			testFunc: func(t *testing.T) {
				ctx := context.WithValue(bctx, key, kad)
				var config = Config{DatabaseURL: "bolt://somedir/overlay.db", StatDBPort: address}
				err := config.Run(ctx, prv)

				assert.Error(t, err)
				if !os.IsNotExist(errs.Unwrap(err)) {
					t.Fatal(err.Error())
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, c.testFunc)
	}
}

func TestUrlPwd(t *testing.T) {
	res := GetUserPassword(nil)

	assert.Equal(t, res, "")

	uinfo := url.UserPassword("testUser", "testPassword")

	uri := url.URL{User: uinfo}

	res = GetUserPassword(&uri)

	assert.Equal(t, res, "testPassword")
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
	lis, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		return nil, "", err
	}
	
	srv := grpc.NewServer()
	err = registerStatDBServer(srv)
	if err != nil {
		return nil, "", err
	}

	go func() {
		srv.Serve(lis)
	}()
	defer lis.Close()
	address := lis.Addr().String()

	ca, err := provider.NewTestCA(ctx)
	if err != nil {
		return nil, "", err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, "", err
	}
	prv, err := provider.NewProvider(identity, lis, nil)
	if err != nil {
		return nil, "", err
	}
	return prv, address, nil
}