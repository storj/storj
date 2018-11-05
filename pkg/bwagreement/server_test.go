// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"fmt"
	"log"
	"net"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/stretchr/testify/assert"

	dbx "storj.io/storj/pkg/bwagreement/dbx"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

var (
	ctx = context.Background()
)

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "your-password"
	dbname   = "pointerdb"
)

func getPSQLInfo() string {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	return psqlInfo
}

func TestBandwidthAgreements(t *testing.T) {
	TS := NewTestServer(t)
	defer TS.Stop()

	signature := []byte("iamthedummysignatureoftypebyteslice")
	data := []byte("iamthedummydataoftypebyteslice")

	msg := &pb.RenterBandwidthAllocation{
		Signature: signature,
		Data:      data,
	}

	/* emulate sending the bwagreement stream from piecestore node */
	stream, err := TS.c.BandwidthAgreements(ctx)
	assert.NoError(t, err)
	err = stream.Send(msg)
	assert.NoError(t, err)

	_, _ = stream.CloseAndRecv()

	/* read back from the postgres db in bwagreement table */
	retData, err := TS.s.DB.Get_Bwagreement_By_Signature(ctx, dbx.Bwagreement_Signature(signature))
	assert.EqualValues(t, retData.Data, data)
	assert.NoError(t, err)

	/* delete the entry what you just wrote */
	delBool, err := TS.s.DB.Delete_Bwagreement_By_Signature(ctx, dbx.Bwagreement_Signature(signature))
	assert.True(t, delBool)
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
	co, err := fiC.DialOption()
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

func newTestServerStruct(t *testing.T) *Server {
	psqlInfo := getPSQLInfo()
	s, err := NewServer("postgres", psqlInfo, zap.NewNop())
	assert.NoError(t, err)
	return s
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
