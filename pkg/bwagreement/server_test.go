// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"flag"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gtank/cryptopasta"
	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/internal/storj"
	"storj.io/storj/pkg/bwagreement/database-manager"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storj"
)

var (
	ctx = context.Background()
)

func TestBandwidthAgreements(t *testing.T) {
	TS := NewTestServer(t)
	defer TS.Stop()

	pba, err := generatePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, TS.k)
	assert.NoError(t, err)

	rba, err := generateRenterBandwidthAllocation(pba, TS.k)
	assert.NoError(t, err)

	/* emulate sending the bwagreement stream from piecestore node */
	_, err = TS.c.BandwidthAgreements(ctx, rba)
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
	co, err := fiC.DialOption(storj.NodeID{})
	check(err)

	s := newTestServerStruct(t, fiC.Key)
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
	defaultPostgresConn = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"
)

var (
	// for travis build support
	testPostgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRES_TEST"), "PostgreSQL test database connection string")
)

func newTestServerStruct(t *testing.T, k crypto.PrivateKey) *Server {
	if *testPostgres == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", defaultPostgresConn)
	}

	dbm, err := dbmanager.NewDBManager("postgres", *testPostgres)
	if err != nil {
		t.Fatalf("Failed to initialize dbmanager when creating test server: %+v", err)
	}

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

func generatePayerBandwidthAllocation(action pb.PayerBandwidthAllocation_Action, satelliteKey crypto.PrivateKey) (*pb.PayerBandwidthAllocation, error) {
	satelliteKeyEcdsa, ok := satelliteKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errs.New("Satellite Private Key is not a valid *ecdsa.PrivateKey")
	}

	// Generate PayerBandwidthAllocation_Data
	data, _ := proto.Marshal(
		&pb.PayerBandwidthAllocation_Data{
			SatelliteId:       teststorj.NodeIDFromString("SatelliteID"),
			UplinkId:          teststorj.NodeIDFromString("UplinkID"),
			ExpirationUnixSec: time.Now().Add(time.Hour * 24 * 10).Unix(),
			SerialNumber:      "SerialNumber",
			Action:            action,
			CreatedUnixSec:    time.Now().Unix(),
		},
	)

	// Sign the PayerBandwidthAllocation_Data with the "Satellite" Private Key
	s, err := cryptopasta.Sign(data, satelliteKeyEcdsa)
	if err != nil {
		return nil, errs.New("Failed to sign PayerBandwidthAllocation_Data with satellite Private Key: %+v", err)
	}

	// Combine Signature and Data for PayerBandwidthAllocation
	return &pb.PayerBandwidthAllocation{
		Data:      data,
		Signature: s,
	}, nil
}

func generateRenterBandwidthAllocation(pba *pb.PayerBandwidthAllocation, uplinkKey crypto.PrivateKey) (*pb.RenterBandwidthAllocation, error) {
	// get "Uplink" Public Key
	uplinkKeyEcdsa, ok := uplinkKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errs.New("Uplink Private Key is not a valid *ecdsa.PrivateKey")
	}

	pubbytes, err := x509.MarshalPKIXPublicKey(&uplinkKeyEcdsa.PublicKey)
	if err != nil {
		return nil, errs.New("Could not generate byte array from Uplink Public key: %+v", err)
	}

	// Generate RenterBandwidthAllocation_Data
	data, _ := proto.Marshal(
		&pb.RenterBandwidthAllocation_Data{
			PayerAllocation: pba,
			PubKey:          pubbytes, // TODO: Take this out. It will be kept in a database on the satellite
			StorageNodeId:   teststorj.NodeIDFromString("StorageNodeID"),
			Total:           int64(666),
		},
	)

	// Sign the PayerBandwidthAllocation_Data with the "Uplink" Private Key
	s, err := cryptopasta.Sign(data, uplinkKeyEcdsa)
	if err != nil {
		return nil, errs.New("Failed to sign RenterBandwidthAllocation_Data with uplink Private Key: %+v", err)
	}

	// Combine Signature and Data for RenterBandwidthAllocation
	return &pb.RenterBandwidthAllocation{
		Signature: s,
		Data:      data,
	}, nil
}

func (TS *TestServer) Stop() {
	if err := TS.conn.Close(); err != nil {
		panic(err)
	}
	TS.grpcs.Stop()
}
