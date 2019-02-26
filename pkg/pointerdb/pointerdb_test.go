// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb_test

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/storage"
	"storj.io/storj/storage/teststore"
)

// mockAPIKeys is mock for api keys store of pointerdb
type mockAPIKeys struct {
	info console.APIKeyInfo
	err  error
}

// GetByKey return api key info for given key
func (keys *mockAPIKeys) GetByKey(ctx context.Context, key console.APIKey) (*console.APIKeyInfo, error) {
	return &keys.info, keys.err
}

func TestServicePut(t *testing.T) {
	validAPIKey := console.APIKey{}
	apiKeys := &mockAPIKeys{}

	for i, tt := range []struct {
		apiKey             []byte
		numOfValidPieces   int
		numOfInvalidPieces int
		err                error
		errString          string
	}{
		{[]byte(validAPIKey.String()), 8, 0, nil, ""},
		{[]byte(validAPIKey.String()), 6, 0, nil, ""},
		{[]byte(validAPIKey.String()), 3, 0, nil, "pointerdb error: Number of valid pieces is lower then repair threshold: 3 < 4"},

		{[]byte(validAPIKey.String()), 4, 4, nil, ""},
		{[]byte(validAPIKey.String()), 3, 5, nil, "pointerdb error: Number of valid pieces is lower then repair threshold: 3 < 4"},

		{[]byte("wrong key"), 1, 0, nil, status.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
		{nil, 8, 0, errors.New("put error"), status.Errorf(codes.Internal, "internal error").Error()},
	} {
		ctx := context.Background()
		ctx = auth.WithAPIKey(ctx, tt.apiKey)

		errTag := fmt.Sprintf("Test case #%d", i)

		log := zaptest.NewLogger(t)
		db := teststore.New()
		service := pointerdb.NewService(log, db)
		s := pointerdb.NewServer(log, service, nil, nil, pointerdb.Config{}, nil, apiKeys)

		path := "a/b/c"
		pointer := makePointer(ctx, t, tt.numOfValidPieces, tt.numOfInvalidPieces)

		if tt.err != nil {
			db.ForceError++
		}

		req := pb.PutRequest{Path: path, Pointer: pointer}
		_, err := s.Put(ctx, &req)

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func makePointer(ctx context.Context, t *testing.T, numOfValidPieces, numOfInvalidPieces int) *pb.Pointer {
	pieces := make([]*pb.RemotePiece, numOfValidPieces+numOfInvalidPieces)
	hashes := make([]*pb.SignedHash, len(pieces))
	for i := 0; i < numOfValidPieces; i++ {
		identity, err := testidentity.NewTestIdentity(ctx)
		assert.NoError(t, err)
		pieces[i] = &pb.RemotePiece{PieceNum: int32(i), NodeId: identity.ID}

		hashes[i] = &pb.SignedHash{Hash: make([]byte, 32)}
		_, err = rand.Read(hashes[i].Hash)
		assert.NoError(t, err)
		err = auth.SignMessage(hashes[i], *identity)
		assert.NoError(t, err)
	}

	// public key did not match expected signer
	for i := numOfValidPieces; i < len(hashes); i++ {
		identity, err := testidentity.NewTestIdentity(ctx)
		assert.NoError(t, err)
		pieces[i] = &pb.RemotePiece{PieceNum: int32(i), NodeId: teststorj.NodeIDFromString(strconv.Itoa(i))}

		hashes[i] = &pb.SignedHash{Hash: make([]byte, 32)}
		_, err = rand.Read(hashes[i].Hash)
		assert.NoError(t, err)
		err = auth.SignMessage(hashes[i], *identity)
		assert.NoError(t, err)
	}

	pointer := &pb.Pointer{
		Type: pb.Pointer_REMOTE,
		Remote: &pb.RemoteSegment{
			Redundancy: &pb.RedundancyScheme{
				MinReq:           2,
				RepairThreshold:  4,
				SuccessThreshold: 6,
				Total:            8,
			},
			RemotePieces:       pieces,
			RemotePiecesHashes: hashes,
		},
	}

	return pointer
}

func TestServiceGet(t *testing.T) {
	ctx := context.Background()
	ca, err := testidentity.NewTestCA(ctx)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	peerCertificates := make([]*x509.Certificate, 2)
	peerCertificates[0] = identity.Leaf
	peerCertificates[1] = identity.CA

	info := credentials.TLSInfo{State: tls.ConnectionState{PeerCertificates: peerCertificates}}

	validAPIKey := console.APIKey{}
	apiKeys := &mockAPIKeys{}
	// creating in-memory db and opening connection
	satdb, err := satellitedb.NewInMemory(zaptest.NewLogger(t))
	defer func() {
		err = satdb.Close()
		assert.NoError(t, err)
	}()
	err = satdb.CreateTables()
	assert.NoError(t, err)

	for i, tt := range []struct {
		apiKey    []byte
		err       error
		errString string
	}{
		{[]byte(validAPIKey.String()), nil, ""},
		{[]byte("wrong key"), nil, status.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
		{nil, errors.New("get error"), status.Errorf(codes.Internal, "internal error").Error()},
	} {
		ctx = auth.WithAPIKey(ctx, tt.apiKey)
		ctx = peer.NewContext(ctx, &peer.Peer{AuthInfo: info})

		errTag := fmt.Sprintf("Test case #%d", i)

		db := teststore.New()
		service := pointerdb.NewService(zap.NewNop(), db)
		allocation := pointerdb.NewAllocationSigner(identity, 45, satdb.CertDB())

		s := pointerdb.NewServer(zap.NewNop(), service, allocation, nil, pointerdb.Config{}, identity, apiKeys)

		path := "a/b/c"

		pr := &pb.Pointer{SegmentSize: 123}
		prBytes, err := proto.Marshal(pr)
		assert.NoError(t, err, errTag)

		_ = db.Put(storage.Key(storj.JoinPaths(apiKeys.info.ProjectID.String(), path)), storage.Value(prBytes))

		if tt.err != nil {
			db.ForceError++
		}

		req := pb.GetRequest{Path: path}
		resp, err := s.Get(ctx, &req)

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
			assert.NoError(t, err, errTag)
			assert.True(t, pb.Equal(pr, resp.Pointer), errTag)

			assert.NotNil(t, resp.GetPba())
		}
	}
}

func TestServiceDelete(t *testing.T) {
	validAPIKey := console.APIKey{}
	apiKeys := &mockAPIKeys{}

	for i, tt := range []struct {
		apiKey    []byte
		err       error
		errString string
	}{
		{[]byte(validAPIKey.String()), nil, ""},
		{[]byte("wrong key"), nil, status.Errorf(codes.Unauthenticated, "Invalid API credential").Error()},
		{nil, errors.New("delete error"), status.Errorf(codes.Internal, "internal error").Error()},
	} {
		ctx := context.Background()
		ctx = auth.WithAPIKey(ctx, tt.apiKey)

		errTag := fmt.Sprintf("Test case #%d", i)

		path := "a/b/c"

		db := teststore.New()
		_ = db.Put(storage.Key(storj.JoinPaths(apiKeys.info.ProjectID.String(), path)), storage.Value("hello"))
		service := pointerdb.NewService(zap.NewNop(), db)
		s := pointerdb.NewServer(zap.NewNop(), service, nil, nil, pointerdb.Config{}, nil, apiKeys)

		if tt.err != nil {
			db.ForceError++
		}

		req := pb.DeleteRequest{Path: path}
		_, err := s.Delete(ctx, &req)

		if err != nil {
			assert.EqualError(t, err, tt.errString, errTag)
		} else {
			assert.NoError(t, err, errTag)
		}
	}
}

func TestServiceList(t *testing.T) {
	validAPIKey := console.APIKey{}
	apiKeys := &mockAPIKeys{}

	db := teststore.New()
	service := pointerdb.NewService(zap.NewNop(), db)
	server := pointerdb.NewServer(zap.NewNop(), service, nil, nil, pointerdb.Config{}, nil, apiKeys)

	pointer := &pb.Pointer{}
	pointer.CreationDate = ptypes.TimestampNow()

	pointerBytes, err := proto.Marshal(pointer)
	if err != nil {
		t.Fatal(err)
	}
	pointerValue := storage.Value(pointerBytes)

	items := []storage.ListItem{
		{Key: storage.Key("sample.😶"), Value: pointerValue},
		{Key: storage.Key("müsic"), Value: pointerValue},
		{Key: storage.Key("müsic/söng1.mp3"), Value: pointerValue},
		{Key: storage.Key("müsic/söng2.mp3"), Value: pointerValue},
		{Key: storage.Key("müsic/album/söng3.mp3"), Value: pointerValue},
		{Key: storage.Key("müsic/söng4.mp3"), Value: pointerValue},
		{Key: storage.Key("ビデオ/movie.mkv"), Value: pointerValue},
	}

	for i := range items {
		items[i].Key = storage.Key(storj.JoinPaths(apiKeys.info.ProjectID.String(), items[i].Key.String()))
	}

	err = storage.PutAll(db, items...)
	if err != nil {
		t.Fatal(err)
	}

	type Test struct {
		APIKey   string
		Request  pb.ListRequest
		Expected *pb.ListResponse
		Error    func(i int, err error)
	}

	// TODO: ZZZ temporarily disabled until endpoint and service split
	// errorWithCode := func(code codes.Code) func(i int, err error) {
	// 	t.Helper()
	// 	return func(i int, err error) {
	// 		t.Helper()
	// 		if status.Code(err) != code {
	// 			t.Fatalf("%d: should fail with %v, got: %v", i, code, err)
	// 		}
	// 	}
	// }

	tests := []Test{
		{
			APIKey:  validAPIKey.String(),
			Request: pb.ListRequest{Recursive: true},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic"},
					{Path: "müsic/album/söng3.mp3"},
					{Path: "müsic/söng1.mp3"},
					{Path: "müsic/söng2.mp3"},
					{Path: "müsic/söng4.mp3"},
					{Path: "sample.😶"},
					{Path: "ビデオ/movie.mkv"},
				},
			},
		}, {
			APIKey:  validAPIKey.String(),
			Request: pb.ListRequest{Recursive: true, MetaFlags: meta.All},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic", Pointer: pointer},
					{Path: "müsic/album/söng3.mp3", Pointer: pointer},
					{Path: "müsic/söng1.mp3", Pointer: pointer},
					{Path: "müsic/söng2.mp3", Pointer: pointer},
					{Path: "müsic/söng4.mp3", Pointer: pointer},
					{Path: "sample.😶", Pointer: pointer},
					{Path: "ビデオ/movie.mkv", Pointer: pointer},
				},
			},
		},
		// { // TODO: ZZZ temporarily disabled until endpoint and service split
		// 	APIKey:  "wrong key",
		// 	Request: pb.ListRequest{Recursive: true, MetaFlags: meta.All}, //, APIKey: []byte("wrong key")},
		// 	Error:   errorWithCode(codes.Unauthenticated),
		// },
		{
			APIKey:  validAPIKey.String(),
			Request: pb.ListRequest{Recursive: true, Limit: 3},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic"},
					{Path: "müsic/album/söng3.mp3"},
					{Path: "müsic/söng1.mp3"},
				},
				More: true,
			},
		}, {
			APIKey:  validAPIKey.String(),
			Request: pb.ListRequest{MetaFlags: meta.All},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic", Pointer: pointer},
					{Path: "müsic/", IsPrefix: true},
					{Path: "sample.😶", Pointer: pointer},
					{Path: "ビデオ/", IsPrefix: true},
				},
				More: false,
			},
		}, {
			APIKey:  validAPIKey.String(),
			Request: pb.ListRequest{EndBefore: "ビデオ"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "müsic"},
					{Path: "müsic/", IsPrefix: true},
					{Path: "sample.😶"},
				},
				More: false,
			},
		}, {
			APIKey:  validAPIKey.String(),
			Request: pb.ListRequest{Recursive: true, Prefix: "müsic/"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "album/söng3.mp3"},
					{Path: "söng1.mp3"},
					{Path: "söng2.mp3"},
					{Path: "söng4.mp3"},
				},
			},
		}, {
			APIKey:  validAPIKey.String(),
			Request: pb.ListRequest{Recursive: true, Prefix: "müsic/", StartAfter: "album/söng3.mp3"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "söng1.mp3"},
					{Path: "söng2.mp3"},
					{Path: "söng4.mp3"},
				},
			},
		}, {
			APIKey:  validAPIKey.String(),
			Request: pb.ListRequest{Prefix: "müsic/"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "album/", IsPrefix: true},
					{Path: "söng1.mp3"},
					{Path: "söng2.mp3"},
					{Path: "söng4.mp3"},
				},
			},
		}, {
			APIKey:  validAPIKey.String(),
			Request: pb.ListRequest{Prefix: "müsic/", StartAfter: "söng1.mp3"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "söng2.mp3"},
					{Path: "söng4.mp3"},
				},
			},
		}, {
			APIKey:  validAPIKey.String(),
			Request: pb.ListRequest{Prefix: "müsic/", EndBefore: "söng4.mp3"},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					{Path: "album/", IsPrefix: true},
					{Path: "söng1.mp3"},
					{Path: "söng2.mp3"},
				},
			},
		}, {
			APIKey:  validAPIKey.String(),
			Request: pb.ListRequest{Prefix: "müs", Recursive: true, EndBefore: "ic/söng4.mp3", Limit: 1},
			Expected: &pb.ListResponse{
				Items: []*pb.ListResponse_Item{
					// {Path: "ic/söng2.mp3"},
				},
				// More: true,
			},
		},
	}

	// TODO:
	//    pb.ListRequest{Prefix: "müsic/", StartAfter: "söng1.mp3", EndBefore: "söng4.mp3"},
	//    failing database
	for i, test := range tests {
		ctx := context.Background()
		ctx = auth.WithAPIKey(ctx, []byte(test.APIKey))

		resp, err := server.List(ctx, &test.Request)
		if test.Error == nil {
			if err != nil {
				t.Fatalf("%d: failed %v", i, err)
			}
		} else {
			test.Error(i, err)
		}

		if diff := cmp.Diff(test.Expected, resp, cmp.Comparer(pb.Equal)); diff != "" {
			t.Errorf("%d: (-want +got) %v\n%s", i, test.Request.String(), diff)
		}
	}
}
