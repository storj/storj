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
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	dbx "storj.io/storj/pkg/bwagreement/dbx"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/rpc/server/psdb"
	"storj.io/storj/pkg/provider"
)

var (
	ctx = context.Background()
)

func TestBandwidthAgreements(t *testing.T) {
	TS := NewTestServer(t)
	defer TS.Stop()

	var signature []byte
	var data []byte

	bwAgreements, err := readSampleDataFromPsdb()
	assert.NoError(t, err)

	/* emulate sending the bwagreement stream from piecestore node */
	stream, err := TS.c.BandwidthAgreements(ctx)
	assert.NoError(t, err)

	for _, v := range bwAgreements {
		for _, j := range v {
			rbad := &pb.RenterBandwidthAllocation_Data{}
			if err := proto.Unmarshal(j.Agreement, rbad); err != nil {
				assert.Error(t, err)
			}
			signature = rbad.GetPayerAllocation().GetSignature()
			data = j.Agreement

			msg := &pb.RenterBandwidthAllocation{
				Signature: signature,
				Data:      j.Agreement,
			}

			err = stream.Send(msg)
			assert.NoError(t, err)

			time.Sleep(1 * time.Millisecond)

			/* read back from the postgres db in bwagreement table */
			retData, err := TS.s.DB.Get_Bwagreement_By_Signature(ctx, dbx.Bwagreement_Signature(signature))
			assert.EqualValues(t, retData.Data, data)
			assert.NoError(t, err)

			/* delete the entry what you just wrote */
			delBool, err := TS.s.DB.Delete_Bwagreement_By_Signature(ctx, dbx.Bwagreement_Signature(signature))
			assert.True(t, delBool)
			assert.NoError(t, err)
		}
	}
	_, _ = stream.CloseAndRecv()
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
	psqlInfo := "postgres://postgres@localhost/pointerdb?sslmode=disable"
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

// call this function to copy signature and data into postgres db
func readSampleDataFromPsdb() (map[string][]*psdb.Agreement, error) {
	// open the sql db
	dbpath := filepath.Join("/Users/kishore/.storj/capt/f37/data", "piecestore.db")

	db, err := psdb.Open(context.Background(), "", dbpath)
	if err != nil {
		fmt.Println("Storagenode database couldnt open:", dbpath)
		return nil, err
	}

	bwAgreements, err := db.GetBandwidthAllocations()
	if err != nil {
		return nil, err
	}

	return bwAgreements, err
}
