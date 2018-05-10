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

func RetrieveShardRequest(conn *grpc.ClientConn, hash string, data io.Writer, length int64, offset int64) (error) {
  c := pb.NewRouteGuideClient(conn)

  stream, err := c.Retrieve(context.Background(), &pb.ShardRetrieval{Hash: hash, Size: length, StoreOffset: offset})
  if err != nil {
    return err
  }

  for {
		shardData, err := stream.Recv()
    if err != nil {
      if err == io.EOF {
        break
      }
      return err
    }

    _, err = data.Write(shardData.Content)
    if err != nil {
      return err
    }

  }

  return nil
}

func DeleteShardRequest(conn *grpc.ClientConn, hash string) (error) {
  c := pb.NewRouteGuideClient(conn)

  reply, err := c.Delete(context.Background(), &pb.ShardDelete{Hash: hash})
  if err != nil {
    return err
  }
  log.Printf("Route summary : %v", reply)
  return nil
}
