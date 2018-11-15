// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"flag"
	"log"
	"net"
	"os"
	"testing"

	"github.com/gtank/cryptopasta"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/bwagreement/database-manager"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/provider"
)

var (
	ctx = context.Background()
)

func TestBandwidthAgreements(t *testing.T) {
	TS := NewTestServer(t)
	defer TS.Stop()

	signature := []byte("iamthedummysignatureoftypebyteslice")
	data := []byte("iamthedummydataoftypebyteslice")

	msg := &pb.RenterBandwidthAllocation{
		Signature: signature,
		Data:      data,
	}

	s, err := cryptopasta.Sign(msg.Data, TS.k.(*ecdsa.PrivateKey))
	assert.NoError(t, err)
	msg.Signature = s

	/* emulate sending the bwagreement stream from piecestore node */
	_, err = TS.c.BandwidthAgreements(ctx, msg)
	assert.NoError(t, err)
}

type TestServer struct {
	s     *Server
	grpcs *grpc.Server
	conn  *grpc.ClientConn
	c     pb.BandwidthClient
	k     crypto.PrivateKey
}

func NewTestServer(t *testing.T) *TestServer {
	check := func(e error) {
		if !assert.NoError(t, e) {
			t.Fail()
		}
	}

	caS, err := provider.NewTestCA(context.Background())
	check(err)
	fiS, err := caS.NewIdentity()
	check(err)
	so, err := fiS.ServerOption()
	check(err)

	caC, err := provider.NewTestCA(context.Background())
	check(err)
	fiC, err := caC.NewIdentity()
	check(err)
	co, err := fiC.DialOption("")
	check(err)

	s := newTestServerStruct(t)
	grpcs := grpc.NewServer(so)

	k, ok := fiC.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	ts := &TestServer{s: s, grpcs: grpcs, k: k}
	addr := ts.start()
	ts.c, ts.conn = connect(addr, co)

	return ts
}

const (
	// this connstring is expected to work under the storj-test docker-compose instance
	defaultPostgresConn = "postgres://pointerdb:pg-secret-pass@test-postgres-pointerdb/pointerdb?sslmode=disable"
)

var (
	// for travis build support
	testPostgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRESKV_TEST"), "PostgreSQL test database connection string")
)

func newTestServerStruct(t *testing.T) *Server {
	if *testPostgres == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", defaultPostgresConn)
	}

	dbm, err := dbmanager.NewDBManager("postgres", *testPostgres)
	if err != nil {
		t.Fatalf("Failed to initialize dbmanager when creating test server: %+v", err)
	}

	k, err := peertls.NewKey()
	assert.NoError(t, err)

	p, _ := k.(*ecdsa.PrivateKey)
	server, err := NewServer(dbm, zap.NewNop(), &p.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func (TS *TestServer) start() (addr string) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	pb.RegisterBandwidthServer(TS.grpcs, TS.s)

	go func() {
		if err := TS.grpcs.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	return lis.Addr().String()
}

func connect(addr string, o ...grpc.DialOption) (pb.BandwidthClient, *grpc.ClientConn) {
	conn, err := grpc.Dial(addr, o...)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	c := pb.NewBandwidthClient(conn)

	return c, conn
}

func (TS *TestServer) Stop() {
	if err := TS.conn.Close(); err != nil {
		panic(err)
	}
	TS.grpcs.Stop()
}
