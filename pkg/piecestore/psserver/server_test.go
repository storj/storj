// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"crypto"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/bwagreement/testbwagreement"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
)

func TestPiece(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	snID, upID := newTestIdentity(ctx, t), newTestIdentity(ctx, t)
	server, client, cleanup := NewTest(ctx, t, snID, upID, []storj.NodeID{})
	defer cleanup()

	namespacedID, err := getNamespacedPieceID([]byte("11111111111111111111"), snID.ID.Bytes())
	require.NoError(t, err)

	if err := writeFile(server, namespacedID); err != nil {
		t.Errorf("Error: %v\nCould not create test piece", err)
		return
	}

	defer func() { _ = server.storage.Delete(namespacedID) }()

	// set up test cases
	tests := []struct {
		id          string
		size        int64
		expiration  int64
		errContains string
	}{
		{ // should successfully retrieve piece meta-data
			id:         "11111111111111111111",
			size:       5,
			expiration: 9999999999,
		},
		{ // server should err with nonexistent file
			id:          "22222222222222222222",
			size:        5,
			expiration:  9999999999,
			errContains: "piecestore error", // TODO: fix for i18n, these message can vary per OS
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			namespacedID, err := getNamespacedPieceID([]byte(tt.id), snID.ID.Bytes())
			require.NoError(t, err)

			// simulate piece TTL entry
			require.NoError(t, server.DB.AddTTL(namespacedID, tt.expiration, tt.size))
			defer func() { require.NoError(t, server.DB.DeleteTTLByID(namespacedID)) }()

			req := &pb.PieceId{Id: tt.id, SatelliteId: snID.ID}
			resp, err := client.Piece(ctx, req)

			if tt.errContains != "" {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.errContains)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, resp)

			assert.Equal(t, tt.id, resp.GetId())
			assert.Equal(t, tt.size, resp.GetPieceSize())
			assert.Equal(t, tt.expiration, resp.GetExpirationUnixSec())
		})
	}
}

func TestRetrieve(t *testing.T) {
	t.Skip("still flaky")

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	snID, upID := newTestIdentity(ctx, t), newTestIdentity(ctx, t)
	server, client, cleanup := NewTest(ctx, t, snID, upID, []storj.NodeID{})
	defer cleanup()

	if err := writeFile(server, "11111111111111111111"); err != nil {
		t.Errorf("Error: %v\nCould not create test piece", err)
		return
	}

	defer func() { _ = server.storage.Delete("11111111111111111111") }()

	// set up test cases
	tests := []struct {
		id          string
		reqSize     int64
		respSize    int64
		allocSize   int64
		offset      int64
		content     []byte
		errContains string
	}{
		{ // should successfully retrieve data
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzwq"),
		},
		{ // should successfully retrieve data in customizeable increments
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  5,
			allocSize: 2,
			offset:    0,
			content:   []byte("xyzwq"),
		},
		{ // should successfully retrieve data with lower allocations
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  3,
			allocSize: 3,
			offset:    0,
			content:   []byte("xyz"),
		},
		{ // should successfully retrieve data
			id:        "11111111111111111111",
			reqSize:   -1,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzwq"),
		},
		{ // server should err with invalid id
			id:          "123",
			reqSize:     5,
			respSize:    5,
			allocSize:   5,
			offset:      0,
			content:     []byte("xyzwq"),
			errContains: "rpc error: code = Unknown desc = piecestore error: invalid id length",
		},
		{ // server should err with nonexistent file
			id:          "22222222222222222222",
			reqSize:     5,
			respSize:    5,
			allocSize:   5,
			offset:      0,
			content:     []byte("xyzwq"),
			errContains: "piecestore error",
		},
		{ // server should return expected content and respSize with offset and excess reqSize
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  4,
			allocSize: 5,
			offset:    1,
			content:   []byte("yzwq"),
		},
		{ // server should return expected content with reduced reqSize
			id:        "11111111111111111111",
			reqSize:   4,
			respSize:  4,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzw"),
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			stream, err := client.Retrieve(ctx)
			require.NoError(t, err)

			// send piece database
			err = stream.Send(&pb.PieceRetrieval{PieceData: &pb.PieceRetrieval_PieceData{Id: tt.id, PieceSize: tt.reqSize, Offset: tt.offset}})
			require.NoError(t, err)

			pba, err := testbwagreement.GenerateOrderLimit(pb.BandwidthAction_GET, snID, upID, time.Hour)
			require.NoError(t, err)

			totalAllocated := int64(0)
			var data string
			var totalRetrieved = int64(0)

			var resp *pb.PieceRetrievalStream
			for totalAllocated < tt.respSize {
				// Send bandwidth bandwidthAllocation
				totalAllocated += tt.allocSize

				rba, err := testbwagreement.GenerateOrder(pba, snID.ID, upID, totalAllocated)
				require.NoError(t, err)

				err = stream.Send(&pb.PieceRetrieval{BandwidthAllocation: rba})
				require.NoError(t, err)

				resp, err = stream.Recv()
				if tt.errContains != "" {
					require.NotNil(t, err)
					require.Contains(t, err.Error(), tt.errContains)
					return
				}
				require.NotNil(t, resp)
				assert.NoError(t, err)

				data = fmt.Sprintf("%s%s", data, string(resp.GetContent()))
				totalRetrieved += resp.GetPieceSize()
			}

			assert.Equal(t, tt.respSize, totalRetrieved)
			assert.Equal(t, string(tt.content), data)
		})
	}
}

func TestStore(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	satID := newTestIdentity(ctx, t)

	tests := []struct {
		id            string
		satelliteID   storj.NodeID
		whitelist     []storj.NodeID
		ttl           int64
		content       []byte
		message       string
		totalReceived int64
		err           string
	}{
		{ // should successfully store data with no approved satellites
			id:            "99999999999999999999",
			satelliteID:   satID.ID,
			whitelist:     []storj.NodeID{},
			ttl:           9999999999,
			content:       []byte("xyzwq"),
			message:       "OK",
			totalReceived: 5,
			err:           "",
		},
		{ // should err with piece ID not specified
			id:            "",
			satelliteID:   satID.ID,
			whitelist:     []storj.NodeID{satID.ID},
			ttl:           9999999999,
			content:       []byte("xyzwq"),
			message:       "",
			totalReceived: 0,
			err:           "rpc error: code = Unknown desc = store error: piece ID not specified",
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			snID, upID := newTestIdentity(ctx, t), newTestIdentity(ctx, t)
			server, client, cleanup := NewTest(ctx, t, snID, upID, tt.whitelist)
			defer cleanup()

			sum := sha256.Sum256(tt.content)
			expectedHash := sum[:]

			stream, err := client.Store(ctx)
			require.NoError(t, err)

			// Create Bandwidth Allocation Data
			pba, err := testbwagreement.GenerateOrderLimit(pb.BandwidthAction_PUT, snID, upID, time.Hour)
			require.NoError(t, err)
			rba, err := testbwagreement.GenerateOrder(pba, snID.ID, upID, tt.totalReceived)
			require.NoError(t, err)

			// Write the buffer to the stream we opened earlier
			err = stream.Send(&pb.PieceStore{
				PieceData:           &pb.PieceStore_PieceData{Id: tt.id, ExpirationUnixSec: tt.ttl},
				BandwidthAllocation: rba,
			})
			require.NoError(t, err)

			msg := &pb.PieceStore{
				PieceData:           &pb.PieceStore_PieceData{Content: tt.content},
				BandwidthAllocation: rba,
			}
			// Write the buffer to the stream we opened earlier
			err = stream.Send(msg)
			if err != io.EOF && err != nil {
				require.NoError(t, err)
			}

			resp, err := stream.CloseAndRecv()
			if tt.err != "" {
				require.Error(t, err)
				require.True(t, strings.HasPrefix(err.Error(), tt.err))
				return
			}

			require.NoError(t, err)
			if assert.NotNil(t, resp) {
				assert.Equal(t, tt.message, resp.Message)
				assert.Equal(t, tt.totalReceived, resp.TotalReceived)
				assert.Equal(t, expectedHash, resp.SignedHash.Hash)
				assert.NotNil(t, resp.SignedHash.Signature)
			}

			allocations, err := server.DB.GetBandwidthAllocationBySignature(rba.Signature)
			require.NoError(t, err)
			require.NotNil(t, allocations)
			for _, allocation := range allocations {
				require.Equal(t, msg.BandwidthAllocation.GetSignature(), allocation.Signature)
				require.Equal(t, int64(len(tt.content)), rba.Total)
			}
		})
	}
}

func TestPbaValidation(t *testing.T) {
	ctx := testcontext.New(t)
	upID1, upID2 := newTestIdentity(ctx, t), newTestIdentity(ctx, t)
	satID1, satID2, satID3 := newTestIdentity(ctx, t), newTestIdentity(ctx, t), newTestIdentity(ctx, t)
	defer ctx.Cleanup()

	tests := []struct {
		satelliteID *identity.FullIdentity
		uplinkID    *identity.FullIdentity
		whitelist   []storj.NodeID
		action      pb.BandwidthAction
		expire      time.Duration
		err         string
	}{
		{ // unapproved satellite id
			satelliteID: satID3,
			uplinkID:    upID1,
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID},
			action:      pb.BandwidthAction_PUT,
			expire:      time.Hour,
			err:         "rpc error: code = Unknown desc = Payer agreement: not signed by any CA in the whitelist",
		},
		{ // approved satellite id
			satelliteID: satID2,
			uplinkID:    upID2,
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID},
			action:      pb.BandwidthAction_PUT,
			expire:      time.Hour,
		},
		{ // wrong action type
			satelliteID: satID1,
			uplinkID:    upID1,
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID},
			action:      pb.BandwidthAction_GET,
			expire:      time.Hour,
			err:         "rpc error: code = Unknown desc = store error: payer bandwidth allocation: invalid action GET",
		},
		{ // expires now
			satelliteID: satID1,
			uplinkID:    upID1,
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID},
			action:      pb.BandwidthAction_GET,
			expire:      0 * time.Second,
			err:         "rpc error: code = Unknown desc = Payer agreement: Agreement is expired:",
		},
		{ // expired yesterday
			satelliteID: satID1,
			uplinkID:    upID1,
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID},
			action:      pb.BandwidthAction_GET,
			expire:      -23*time.Hour - 55*time.Second,
			err:         "rpc error: code = Unknown desc = Payer agreement: Agreement is expired:",
		},
		{ // expires in 30 seconds
			satelliteID: satID1,
			uplinkID:    upID1,
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID},
			action:      pb.BandwidthAction_GET,
			expire:      30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			server, client, cleanup := NewTest(ctx, t, tt.satelliteID, tt.uplinkID, tt.whitelist)
			defer cleanup()

			stream, err := client.Store(ctx)
			require.NoError(t, err)

			// Create Bandwidth Allocation Data
			content := []byte("content")
			pba, err := testbwagreement.GenerateOrderLimit(tt.action, tt.satelliteID, tt.uplinkID, tt.expire)
			require.NoError(t, err)
			rba, err := testbwagreement.GenerateOrder(pba, tt.satelliteID.ID, tt.uplinkID, int64(len(content)))
			require.NoError(t, err)
			msg := &pb.PieceStore{
				PieceData:           &pb.PieceStore_PieceData{Content: content},
				BandwidthAllocation: rba,
			}

			//cleanup incase tests previously paniced
			_ = server.storage.Delete("99999999999999999999")
			// Write the buffer to the stream we opened earlier
			err = stream.Send(&pb.PieceStore{
				PieceData:           &pb.PieceStore_PieceData{Id: "99999999999999999999", ExpirationUnixSec: 9999999999},
				BandwidthAllocation: rba,
			})
			require.NoError(t, err)

			// Write the buffer to the stream we opened earlier
			err = stream.Send(msg)
			if err != io.EOF && err != nil {
				require.NoError(t, err)
			}

			_, err = stream.CloseAndRecv()
			if tt.err != "" {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.err)
				return
			}
		})
	}
}

func TestDelete(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	snID, upID := newTestIdentity(ctx, t), newTestIdentity(ctx, t)
	server, client, cleanup := NewTest(ctx, t, snID, upID, []storj.NodeID{})
	defer cleanup()

	pieceID := "11111111111111111111"
	namespacedID, err := getNamespacedPieceID([]byte(pieceID), snID.ID.Bytes())
	require.NoError(t, err)

	// simulate piece stored with storagenode
	if err := writeFile(server, namespacedID); err != nil {
		t.Errorf("Error: %v\nCould not create test piece", err)
		return
	}
	require.NoError(t, server.DB.AddTTL(namespacedID, 1234567890, 1234567890))
	defer func() { require.NoError(t, server.DB.DeleteTTLByID(namespacedID)) }()

	resp, err := client.Delete(ctx, &pb.PieceDelete{
		Id:          pieceID,
		SatelliteId: snID.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "OK", resp.GetMessage())

	// check if file was indeed deleted
	_, err = server.storage.Size(namespacedID)
	require.Error(t, err)

	resp, err = client.Delete(ctx, &pb.PieceDelete{
		Id:          "22222222222222",
		SatelliteId: snID.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "OK", resp.GetMessage())
}

func NewTest(ctx context.Context, t *testing.T, snID, upID *identity.FullIdentity, ids []storj.NodeID) (*Server, pb.PieceStoreRoutesClient, func()) {

	//init ps server backend
	tmp, err := ioutil.TempDir("", "storj-piecestore")
	require.NoError(t, err)

	tempDBPath := filepath.Join(tmp, "test.db")
	tempDir := filepath.Join(tmp, "test-data", "3000")

	storage := pstore.NewStorage(tempDir)

	psDB, err := psdb.Open(tempDBPath)
	require.NoError(t, err)

	err = psDB.Migration().Run(zaptest.NewLogger(t), psDB)
	require.NoError(t, err)

	whitelist := make(map[storj.NodeID]crypto.PublicKey)
	for _, id := range ids {
		whitelist[id] = nil
	}

	psServer := &Server{
		log:              zaptest.NewLogger(t),
		storage:          storage,
		DB:               psDB,
		identity:         snID,
		totalAllocated:   math.MaxInt64,
		totalBwAllocated: math.MaxInt64,
		whitelist:        whitelist,
	}

	//init ps server grpc
	sc := server.Config{
		Address: "127.0.0.1:0",
		PrivateAddress: "127.0.0.1:0",
		Config: tlsopts.Config{
			PeerIDVersions: "1,2",
		},
	}
	options, err := tlsopts.NewOptions(snID, sc.Config)
	require.NoError(t, err)

	grpcServer, err := server.New(options, sc.Address, sc.PrivateAddress, nil)
	require.NoError(t, err)

	pb.RegisterPieceStoreRoutesServer(grpcServer.GRPC(), psServer)
	go func() { require.NoError(t, grpcServer.Run(ctx)) }() // TODO: wait properly for server termination
	//init client

	tlsOptions, err := tlsopts.NewOptions(upID, tlsopts.Config{
		PeerIDVersions: "1,2",
	})
	require.NoError(t, err)

	// TODO: why aren't we using transport client here?
	conn, err := grpc.Dial(grpcServer.Addr().String(), tlsOptions.DialUnverifiedIDOption())
	require.NoError(t, err)
	psClient := pb.NewPieceStoreRoutesClient(conn)
	//cleanup callback
	cleanup := func() {
		require.NoError(t, conn.Close())
		require.NoError(t, psServer.Close())
		require.NoError(t, psServer.Stop(ctx))
		require.NoError(t, os.RemoveAll(tmp))
	}
	return psServer, psClient, cleanup
}

func newTestIdentity(ctx context.Context, t *testing.T) *identity.FullIdentity {
	id, err := testidentity.NewTestIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func writeFile(s *Server, pieceID string) error {
	file, err := s.storage.Writer(pieceID)
	if err != nil {
		return err
	}
	_, err = file.Write([]byte("xyzwq"))
	return errs.Combine(err, file.Close())
}
