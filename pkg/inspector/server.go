package inspector

import (
	"context"
	"fmt"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
)

var (
	// ServerError is a gRPC server error for Inspector
	ServerError = errs.Class("inspector server error:")
)

// Server holds references to cache and kad
type Server struct {
	dht     dht.DHT
	cache   *overlay.Cache
	logger  *zap.Logger
	metrics *monkit.Registry
}

// CountNodes returns the number of nodes in the cache and in kademlia
func (srv *Server) CountNodes(ctx context.Context, req *pb.CountNodesRequest) (*pb.CountNodesResponse, error) {
	fmt.Printf("Count Nodes Method hit")
	return &pb.CountNodesResponse{
		Kademlia: 0,
		Overlay:  0,
	}, nil
}

// GetBuckets returns all kademlia buckets for current kademlia instance
func (srv *Server) GetBuckets(ctx context.Context, req *pb.GetBucketsRequest) (*pb.GetBucketsResponse, error) {
	fmt.Printf("GetBuckets method hit")
	var buckets []*pb.Bucket
	return &pb.GetBucketsResponse{
		Buckets: buckets,
	}, nil
}

func (srv *Server) GetBucket(ctx context.Context, req *pb.GetBucketRequest) (*pb.GetBucketResponse, error) {
	fmt.Printf("GetBucket request")
	return &pb.GetBucketResponse{}, nil
}
