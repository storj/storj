package api

import (
  "io"
  "log"

  "golang.org/x/net/context"

  "google.golang.org/grpc"

  "github.com/zeebo/errs"

  pb "storj.io/storj/examples/piecestore/rpc/protobuf"
)

var ServerError = errs.Class("serverError")

// Struct containing Shard information from ShardMetaRequest
type ShardMeta struct {
  Hash       string
  Size       int64
  Expiration int64
}

// Request info about a shard by Shard Hash
func ShardMetaRequest(conn *grpc.ClientConn, hash string) (*ShardMeta, error) {
  c := pb.NewRouteGuideClient(conn)

  reply, err := c.Shard(context.Background(), &pb.ShardHash{Hash: hash})
  if err != nil {
    return nil, err
  }

  return &ShardMeta{Hash: reply.Hash, Size: reply.Size, Expiration: reply.Expiration}, nil
}

// Upload Shard to Server
func StoreShardRequest(conn *grpc.ClientConn, hash string, data io.Reader, dataOffset int64, length int64, ttl int64, storeOffset int64) (error) {
  c := pb.NewRouteGuideClient(conn)

  stream, err := c.Store(context.Background())

  buffer := make([]byte, 4096)
  for {
    // Read data from read stream into buffer
    n, err := data.Read(buffer)
    if err == io.EOF {
      break
    }

    // Write the buffer to the stream we opened earlier
    if err := stream.Send(&pb.ShardStore{Hash: hash, Size: length, Ttl: ttl, StoreOffset: storeOffset, Content: buffer[:n]}); err != nil {
  		log.Fatalf("%v.Send() = %v", stream, err)
  	}
  }

  reply, err := stream.CloseAndRecv()
  if err != nil {
    return err
  }

  log.Printf("Route summary: %v", reply)

  if reply.Status != 0 {
    return ServerError.New(reply.Message)
  }

  return nil
}

// Struct for reading shard download stream from server
type ShardStreamReader struct {
  stream pb.RouteGuide_RetrieveClient
}

// Read method for shard download stream
func (s *ShardStreamReader) Read(b []byte) (int, error) {
  shardData, err := s.stream.Recv()
  if err != nil {
    return 0, err
  }

  n := copy(b, shardData.Content)
  return n, err
}

// Begin Download Shard from Server
func RetrieveShardRequest(conn *grpc.ClientConn, hash string, length int64, offset int64) (io.Reader, error) {
  c := pb.NewRouteGuideClient(conn)

  stream, err := c.Retrieve(context.Background(), &pb.ShardRetrieval{Hash: hash, Size: length, StoreOffset: offset})
  if err != nil {
    return nil, err
  }

  return &ShardStreamReader{stream: stream}, err
}

// Delete Shard From Server
func DeleteShardRequest(conn *grpc.ClientConn, hash string) (error) {
  c := pb.NewRouteGuideClient(conn)

  reply, err := c.Delete(context.Background(), &pb.ShardDelete{Hash: hash})
  if err != nil {
    return err
  }
  log.Printf("Route summary : %v", reply)
  return nil
}
