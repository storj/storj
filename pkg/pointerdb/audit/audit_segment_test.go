package audit

import (
	"context"
	"crypto/rand"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/storage/teststore"
	"storj.io/storj/storage/redis/redisserver"

)

const (
	noList    = "list error: failed to get list"
	noNum     = "num error: failed to get num"
)

var (
	ctx          = context.Background()
	ErrNoList    = errors.New(noList)
	ErrorNoNum   = errors.New(noNum)
)

// The client and server implementation are different;
// This is a  wrapper so the pointerdb client can be implemented
//R***********R***********/PointerDB Wrapper/***********R***********R********//
type pointerDBWrapper struct {
	s pb.PointerDBServer
}

func (pbd *pointerDBWrapper) Put(ctx context.Context, in *pb.PutRequest, opts ...grpc.CallOption) (*pb.PutResponse, error) {
	return pbd.s.Put(ctx, in)
}

func (pbd *pointerDBWrapper) Get(ctx context.Context, in *pb.GetRequest, opts ...grpc.CallOption) (*pb.GetResponse, error) {
	return pbd.s.Get(ctx, in)
}

func (pbd *pointerDBWrapper) List(ctx context.Context, in *pb.ListRequest, opts ...grpc.CallOption) (*pb.ListResponse, error) {
	return pbd.s.List(ctx, in)
}

func (pbd *pointerDBWrapper) Delete(ctx context.Context, in *pb.DeleteRequest, opts ...grpc.CallOption) (*pb.DeleteResponse, error) {
	return pbd.s.Delete(ctx, in)
}

func newPointerDBWrapper(pdbs pb.PointerDBServer) pb.PointerDBClient {
	return &pointerDBWrapper{pdbs}
}

type pathCount struct {
	path  paths.Path
	count int
}

func TestAuditSegment(t *testing.T) {
	// note: to simulate better,
	// change limit in library to 5 in
	// list api call, default is  0 == 1000 listing
	tests := []struct {
		bm     string
		path   paths.Path
		APIKey []byte
	}{
		{
			bm:     "success-1",
			path:   paths.New("folder1/file1"),
			APIKey: nil,
		},
		{
			bm:     "success-2",
			path:   paths.New("foodFolder1/file1/file2"),
			APIKey: nil,
		},
		{
			bm:     "success-3",
			path:   paths.New("foodFolder1/file1/file2/foodFolder2/file3"),
			APIKey: nil,
		},
		{
			bm:     "success-4",
			path:   paths.New("projectFolder/project1.txt/"),
			APIKey: nil,
		},
		{
			bm:     "success-5",
			path:   paths.New("newProjectFolder/project2.txt"),
			APIKey: nil,
		},
		{
			bm:     "success-6",
			path:   paths.New("Pictures/image1.png"),
			APIKey: nil,
		},
		{
			bm:     "success-7",
			path:   paths.New("Pictures/Nature/mountains.png"),
			APIKey: nil,
		},
		{
			bm:     "success-8",
			path:   paths.New("Pictures/City/streets.png"),
			APIKey: nil,
		},
		{
			bm:     "success-9",
			path:   paths.New("Pictures/Animals/Dogs/dogs.png"),
			APIKey: nil,
		},
		{
			bm:     "success-10",
			path:   paths.New("Nada/ビデオ/😶"),
			APIKey: nil,
		},
	}

	//PointerDB instantation
	db := teststore.New()
	c := pointerdb.Config{MaxInlineSegmentSize: 8000}
	
	redisAddr, cleanup, err := redisserver.Start()
	if err != nil {
		t.Fatal(err)
	}
	
	defer cleanup()

	cache, err := overlay.NewRedisOverlayCache(redisAddr, "", 1, nil)

	assert.NoError(t, err)
	assert.NotNil(t, cache)

	pdbw := newPointerDBWrapper(pointerdb.NewServer(db, cache, zap.NewNop(), c))
	pointers := pdbclient.New(pdbw, nil)

	// create a pdb client and instance of audit
	a := NewAudit(pointers)

	// put 10 paths in db
	t.Run("putToDB", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.bm, func(t *testing.T) {
				assert1 := assert.New(t)

				// create a pointer and put in db
				putRequest := makePointer(tt.path, tt.APIKey)

				// create putreq. object
				req := &pb.PutRequest{Path: tt.path.String(), Pointer: putRequest.Pointer, APIKey: tt.APIKey}

				//Put pointer into db
				_, err := pdbw.Put(ctx, req)
				if err != nil {
					t.Fatalf("failed to put %v: error: %v", req.Pointer, err)
					assert1.NotNil(err)
				}
				if err != nil {
					t.Error("cant instantiate the piece store client")
				}
			})
		}
	})

	t.Run("NextStripe", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.bm, func(t *testing.T) {
				assert1 := assert.New(t)
				stripe, _, err := a.NextStripe(ctx)
				if err != nil {
					assert1.Error(err)
					assert1.Nil(stripe)
				}
				if stripe != nil {
					assert1.Nil(err)
				}
			})
		}
	})

	// test to see how random paths are
	t.Run("probalisticTest", func(t *testing.T) {
		list, _, err := pointers.List(ctx, nil, nil, nil, true, 10, meta.None)
		if err != nil {
			t.Error(ErrNoList)
		}

		// get count of items picked at random
		uniquePathCounted := []pathCount{}
		pathCounter := []pathCount{}

		// get a list of 100 paths generated from random
		for i := 0; i < 100; i++ {
			randomNum, err := rand.Int(rand.Reader, big.NewInt(int64(len(list))))
			if err != nil {
				t.Error(ErrorNoNum)
			}
			pointerItem := list[randomNum.Int64()]
			path := pointerItem.Path
			val := pathCount{path: path, count: 1}
			pathCounter = append(pathCounter, val)
		}

		// get a count for paths in list
		for _, pc := range pathCounter {
			skip := false
			for i, up := range uniquePathCounted {
				if reflect.DeepEqual(pc.path, up.path) {
					up.count = up.count + 1
					uniquePathCounted[i] = up
					skip = true
					break
				}
			}
			if !skip {
				uniquePathCounted = append(uniquePathCounted, pc)
			}
		}

		// Section: binomial test for randomness
		n := float64(100) // events
		p := float64(.10) // theoretical probability of getting  1/10 paths
		m := n * p
		s := math.Sqrt(m * (1 - p)) // binomial distribution

		// if values fall outside of the critical values of test statistics (ie Z value)
		// in a 2-tail test
		// we can assume, 95% confidence, it's not sampling according to a 10% probability
		for _, v := range uniquePathCounted {
			z := (float64(v.count) - m) / s
			if z <= -1.96 || z >= 1.96 {
				t.Log(false)
			} else {
				t.Log(true)
			}
		}
	})
}

func makePointer(path paths.Path, auth []byte) pb.PutRequest {
	var rps []*pb.RemotePiece
	rps = append(rps, &pb.RemotePiece{
		PieceNum: 1,
		NodeId:   "testId",
	})
	pr := pb.PutRequest{
		Path: path.String(),
		Pointer: &pb.Pointer{
			Type: pb.Pointer_REMOTE,
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					Type:             pb.RedundancyScheme_RS,
					MinReq:           1,
					Total:            3,
					RepairThreshold:  2,
					SuccessThreshold: 3,
					ErasureShareSize: 2,
				},
				PieceId:      "testId",
				RemotePieces: rps,
			},
			Size: int64(10),
		},
		APIKey: auth,
	}
	return pr
}
