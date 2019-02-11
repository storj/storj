// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"crypto"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	_ "github.com/mattn/go-sqlite3"
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
	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
)

func TestPiece(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	snID, upID := newTestID(ctx, t), newTestID(ctx, t)
	s, c, cleanup := NewTest(ctx, t, snID, upID, []storj.NodeID{})
	defer cleanup()

	if err := writeFile(s, "11111111111111111111"); err != nil {
		t.Errorf("Error: %v\nCould not create test piece", err)
		return
	}

	defer func() { _ = s.storage.Delete("11111111111111111111") }()

	// set up test cases
	tests := []struct {
		id         string
		size       int64
		expiration int64
		err        string
	}{
		{ // should successfully retrieve piece meta-data
			id:         "11111111111111111111",
			size:       5,
			expiration: 9999999999,
			err:        "",
		},
		{ // server should err with invalid id
			id:         "123",
			size:       5,
			expiration: 9999999999,
			err:        "rpc error: code = Unknown desc = piecestore error: invalid id length",
		},
		{ // server should err with nonexistent file
			id:         "22222222222222222222",
			size:       5,
			expiration: 9999999999,
			err: fmt.Sprintf("rpc error: code = Unknown desc = stat %s: no such file or directory", func() string {
				path, _ := s.storage.PiecePath("22222222222222222222")
				return path
			}()),
		},
		{ // server should err with invalid TTL
			id:         "22222222222222222222;DELETE*FROM TTL;;;;",
			size:       5,
			expiration: 9999999999,
			err:        "rpc error: code = Unknown desc = PSServer error: invalid ID",
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			// simulate piece TTL entry
			_, err := s.DB.DB.Exec(fmt.Sprintf(`INSERT INTO ttl (id, created, expires) VALUES ("%s", "%d", "%d")`, tt.id, 1234567890, tt.expiration))
			require.NoError(t, err)

			defer func() {
				_, err := s.DB.DB.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE id="%s"`, tt.id))
				require.NoError(t, err)
			}()

			req := &pb.PieceId{Id: tt.id}
			resp, err := c.Piece(ctx, req)

			if tt.err != "" {
				require.NotNil(t, err)
				if runtime.GOOS == "windows" && strings.Contains(tt.err, "no such file or directory") {
					//TODO (windows): ignoring for windows due to different underlying error
					return
				}
				require.Equal(t, tt.err, err.Error())
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
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	snID, upID := newTestID(ctx, t), newTestID(ctx, t)
	s, c, cleanup := NewTest(ctx, t, snID, upID, []storj.NodeID{})
	defer cleanup()

	if err := writeFile(s, "11111111111111111111"); err != nil {
		t.Errorf("Error: %v\nCould not create test piece", err)
		return
	}

	defer func() { _ = s.storage.Delete("11111111111111111111") }()

	// set up test cases
	tests := []struct {
		id        string
		reqSize   int64
		respSize  int64
		allocSize int64
		offset    int64
		content   []byte
		err       string
	}{
		{ // should successfully retrieve data
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzwq"),
			err:       "",
		},
		{ // should successfully retrieve data in customizeable increments
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  5,
			allocSize: 2,
			offset:    0,
			content:   []byte("xyzwq"),
			err:       "",
		},
		{ // should successfully retrieve data with lower allocations
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  3,
			allocSize: 3,
			offset:    0,
			content:   []byte("xyz"),
			err:       "",
		},
		{ // should successfully retrieve data
			id:        "11111111111111111111",
			reqSize:   -1,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzwq"),
			err:       "",
		},
		{ // server should err with invalid id
			id:        "123",
			reqSize:   5,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzwq"),
			err:       "rpc error: code = Unknown desc = piecestore error: invalid id length",
		},
		{ // server should err with nonexistent file
			id:        "22222222222222222222",
			reqSize:   5,
			respSize:  5,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzwq"),
			err: fmt.Sprintf("rpc error: code = Unknown desc = retrieve error: stat %s: no such file or directory", func() string {
				path, _ := s.storage.PiecePath("22222222222222222222")
				return path
			}()),
		},
		{ // server should return expected content and respSize with offset and excess reqSize
			id:        "11111111111111111111",
			reqSize:   5,
			respSize:  4,
			allocSize: 5,
			offset:    1,
			content:   []byte("yzwq"),
			err:       "",
		},
		{ // server should return expected content with reduced reqSize
			id:        "11111111111111111111",
			reqSize:   4,
			respSize:  4,
			allocSize: 5,
			offset:    0,
			content:   []byte("xyzw"),
			err:       "",
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			stream, err := c.Retrieve(ctx)
			require.NoError(t, err)

			// send piece database
			err = stream.Send(&pb.PieceRetrieval{PieceData: &pb.PieceRetrieval_PieceData{Id: tt.id, PieceSize: tt.reqSize, Offset: tt.offset}})
			require.NoError(t, err)

			pba, err := testbwagreement.GeneratePayerBandwidthAllocation(pb.BandwidthAction_GET, snID, upID, time.Hour)
			require.NoError(t, err)

			totalAllocated := int64(0)
			var data string
			var totalRetrieved = int64(0)
			var resp *pb.PieceRetrievalStream
			for totalAllocated < tt.respSize {
				// Send bandwidth bandwidthAllocation
				totalAllocated += tt.allocSize

				rba, err := testbwagreement.GenerateRenterBandwidthAllocation(pba, snID.ID, upID, totalAllocated)
				require.NoError(t, err)

				err = stream.Send(&pb.PieceRetrieval{BandwidthAllocation: rba})
				require.NoError(t, err)

				resp, err = stream.Recv()
				if tt.err != "" {
					require.NotNil(t, err)
					if runtime.GOOS == "windows" && strings.Contains(tt.err, "no such file or directory") {
						//TODO (windows): ignoring for windows due to different underlying error
						return
					}
					require.Equal(t, tt.err, err.Error())
					return
				}
				assert.NoError(t, err)

				data = fmt.Sprintf("%s%s", data, string(resp.GetContent()))
				totalRetrieved += resp.GetPieceSize()
			}

			assert.NoError(t, err)
			require.NotNil(t, resp)

			assert.Equal(t, tt.respSize, totalRetrieved)
			assert.Equal(t, string(tt.content), data)
		})
	}
}

func TestStore(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	satID := newTestID(ctx, t)

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
		{ // should err with invalid id length
			id:            "butts",
			satelliteID:   satID.ID,
			whitelist:     []storj.NodeID{satID.ID},
			ttl:           9999999999,
			content:       []byte("xyzwq"),
			message:       "",
			totalReceived: 0,
			err:           "rpc error: code = Unknown desc = piecestore error: invalid id length",
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
			snID, upID := newTestID(ctx, t), newTestID(ctx, t)
			s, c, cleanup := NewTest(ctx, t, snID, upID, tt.whitelist)
			defer cleanup()
			db := s.DB.DB

			stream, err := c.Store(ctx)
			require.NoError(t, err)

			// Write the buffer to the stream we opened earlier
			err = stream.Send(&pb.PieceStore{PieceData: &pb.PieceStore_PieceData{Id: tt.id, ExpirationUnixSec: tt.ttl}})
			require.NoError(t, err)
			// Send Bandwidth Allocation Data
			pba, err := testbwagreement.GeneratePayerBandwidthAllocation(pb.BandwidthAction_PUT, snID, upID, time.Hour)
			require.NoError(t, err)
			rba, err := testbwagreement.GenerateRenterBandwidthAllocation(pba, snID.ID, upID, tt.totalReceived)
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

			defer func() {
				_, err := db.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE id="%s"`, tt.id))
				require.NoError(t, err)
			}()

			// check db to make sure agreement and signature were stored correctly
			rows, err := db.Query(`SELECT agreement, signature FROM bandwidth_agreements`)
			require.NoError(t, err)

			defer func() { require.NoError(t, rows.Close()) }()
			for rows.Next() {
				var agreement, signature []byte
				err = rows.Scan(&agreement, &signature)
				require.NoError(t, err)
				rba := &pb.RenterBandwidthAllocation{}
				require.NoError(t, proto.Unmarshal(agreement, rba))
				require.Equal(t, msg.BandwidthAllocation.GetSignature(), signature)
				require.True(t, pb.Equal(pba, &rba.PayerAllocation))
				require.Equal(t, int64(len(tt.content)), rba.Total)

			}
			err = rows.Err()
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, tt.message, resp.Message)
			require.Equal(t, tt.totalReceived, resp.TotalReceived)
		})
	}
}

func TestPbaValidation(t *testing.T) {
	ctx := testcontext.New(t)
	snID, upID := newTestID(ctx, t), newTestID(ctx, t)
	satID1, satID2, satID3 := newTestID(ctx, t), newTestID(ctx, t), newTestID(ctx, t)
	defer ctx.Cleanup()

	tests := []struct {
		satelliteID storj.NodeID
		uplinkID    storj.NodeID
		whitelist   []storj.NodeID
		action      pb.BandwidthAction
		err         string
	}{
		{ // unapproved satellite id
			satelliteID: satID1.ID,
			uplinkID:    upID.ID,
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID, satID3.ID},
			action:      pb.BandwidthAction_PUT,
			err:         "rpc error: code = Unknown desc = store error: Satellite ID not approved",
		},
		{ // missing satellite id
			satelliteID: storj.NodeID{},
			uplinkID:    upID.ID,
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID, satID3.ID},
			action:      pb.BandwidthAction_PUT,
			err:         "rpc error: code = Unknown desc = store error: payer bandwidth allocation: missing satellite id",
		},
		{ // missing uplink id
			satelliteID: satID1.ID,
			uplinkID:    storj.NodeID{},
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID, satID3.ID},
			action:      pb.BandwidthAction_PUT,
			err:         "rpc error: code = Unknown desc = store error: payer bandwidth allocation: missing uplink id",
		},
		{ // wrong action type
			satelliteID: satID1.ID,
			uplinkID:    upID.ID,
			whitelist:   []storj.NodeID{satID1.ID, satID2.ID, satID3.ID},
			action:      pb.BandwidthAction_GET,
			err:         "rpc error: code = Unknown desc = store error: payer bandwidth allocation: invalid action GET",
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			s, c, cleanup := NewTest(ctx, t, snID, upID, tt.whitelist)
			defer cleanup()

			stream, err := c.Store(ctx)
			require.NoError(t, err)

			//cleanup incase tests previously paniced
			_ = s.storage.Delete("99999999999999999999")
			// Write the buffer to the stream we opened earlier
			err = stream.Send(&pb.PieceStore{PieceData: &pb.PieceStore_PieceData{Id: "99999999999999999999", ExpirationUnixSec: 9999999999}})
			require.NoError(t, err)
			// Send Bandwidth Allocation Data
			content := []byte("content")
			pba, err := testbwagreement.GeneratePayerBandwidthAllocation(tt.action, satID1, upID, time.Hour)
			require.NoError(t, err)
			rba, err := testbwagreement.GenerateRenterBandwidthAllocation(pba, snID.ID, upID, int64(len(content)))
			require.NoError(t, err)
			msg := &pb.PieceStore{
				PieceData:           &pb.PieceStore_PieceData{Content: content},
				BandwidthAllocation: rba,
			}

			// Write the buffer to the stream we opened earlier
			err = stream.Send(msg)
			if err != io.EOF && err != nil {
				require.NoError(t, err)
			}

			_, err = stream.CloseAndRecv()
			if err != nil {
				//require.NotNil(t, err)
				t.Log("Expected err string", tt.err)
				t.Log("Actual err.Error:", err.Error())
				require.Equal(t, tt.err, err.Error())
				return
			}
		})
	}
}

func TestDelete(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	snID, upID := newTestID(ctx, t), newTestID(ctx, t)
	s, c, cleanup := NewTest(ctx, t, snID, upID, []storj.NodeID{})
	defer cleanup()

	db := s.DB.DB

	// set up test cases
	tests := []struct {
		id      string
		message string
		err     string
	}{
		{ // should successfully delete data
			id:      "11111111111111111111",
			message: "OK",
			err:     "",
		},
		{ // should err with invalid id length
			id:      "123",
			message: "rpc error: code = Unknown desc = piecestore error: invalid id length",
			err:     "rpc error: code = Unknown desc = piecestore error: invalid id length",
		},
		{ // should return OK with nonexistent file
			id:      "22222222222222222223",
			message: "OK",
			err:     "",
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			// simulate piece stored with storagenode
			if err := writeFile(s, "11111111111111111111"); err != nil {
				t.Errorf("Error: %v\nCould not create test piece", err)
				return
			}

			// simulate piece TTL entry
			_, err := db.Exec(fmt.Sprintf(`INSERT INTO ttl (id, created, expires) VALUES ("%s", "%d", "%d")`, tt.id, 1234567890, 1234567890))
			require.NoError(t, err)

			defer func() {
				_, err := db.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE id="%s"`, tt.id))
				require.NoError(t, err)
			}()

			defer func() {
				require.NoError(t, s.storage.Delete("11111111111111111111"))
			}()

			req := &pb.PieceDelete{Id: tt.id}
			resp, err := c.Delete(ctx, req)

			if tt.err != "" {
				require.Equal(t, tt.err, err.Error())
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.message, resp.GetMessage())

			// if test passes, check if file was indeed deleted
			filePath, err := s.storage.PiecePath(tt.id)
			require.NoError(t, err)
			if _, err = os.Stat(filePath); os.IsExist(err) {
				t.Errorf("File not deleted")
				return
			}
		})
	}
}

func NewTest(ctx context.Context, t *testing.T, snID, upID *identity.FullIdentity,
	ids []storj.NodeID) (*Server, pb.PieceStoreRoutesClient, func()) {
	//init ps server backend
	tmp, err := ioutil.TempDir("", "storj-piecestore")
	require.NoError(t, err)
	tempDBPath := filepath.Join(tmp, "test.db")
	tempDir := filepath.Join(tmp, "test-data", "3000")
	storage := pstore.NewStorage(tempDir)
	psDB, err := psdb.Open(tempDBPath)
	require.NoError(t, err)
	verifier := func(authorization *pb.SignedMessage) error {
		return nil
	}
	whitelist := make(map[storj.NodeID]crypto.PublicKey)
	for _, id := range ids {
		whitelist[id] = nil
	}
	psServer := &Server{
		log:              zaptest.NewLogger(t),
		storage:          storage,
		DB:               psDB,
		verifier:         verifier,
		totalAllocated:   math.MaxInt64,
		totalBwAllocated: math.MaxInt64,
		whitelist:        whitelist,
	}
	//init ps server grpc
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	publicConfig := server.Config{Address: "127.0.0.1:0"}
	publicOptions, err := server.NewOptions(snID, publicConfig)
	require.NoError(t, err)
	grpcServer, err := server.New(publicOptions, listener, nil)
	require.NoError(t, err)
	pb.RegisterPieceStoreRoutesServer(grpcServer.GRPC(), psServer)
	go func() { require.NoError(t, grpcServer.Run(ctx)) }()
	//init client
	co, err := upID.DialOption(storj.NodeID{})
	require.NoError(t, err)
	conn, err := grpc.Dial(listener.Addr().String(), co)
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

func newTestID(ctx context.Context, t *testing.T) *identity.FullIdentity {
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
