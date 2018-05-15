package api

import (
  "io"
  "log"

  "golang.org/x/net/context"

  "github.com/zeebo/errs"

  pb "storj.io/storj/examples/piecestore/rpc/protobuf"
)

var ServerError = errs.Class("serverError")

// Struct containing Piece information from PieceMetaRequest
type PieceMeta struct {
  Hash       string
  Size       int64
  Expiration int64
}

// Request info about a piece by Hash
func PieceMetaRequest(c pb.PieceStoreRoutesClient, hash string) (*PieceMeta, error) {

  reply, err := c.Piece(context.Background(), &pb.PieceHash{Hash: hash})
  if err != nil {
    return nil, err
  }

  return &PieceMeta{Hash: reply.Hash, Size: reply.Size, Expiration: reply.Expiration}, nil
}

// Upload Piece to Server
func StorePieceRequest(c pb.PieceStoreRoutesClient, hash string, data io.Reader, dataOffset int64, length int64, ttl int64, storeOffset int64) (error) {
  stream, err := c.Store(context.Background())

  buffer := make([]byte, 4096)
  for {
    // Read data from read stream into buffer
    n, err := data.Read(buffer)
    if err != nil {
      if err == io.EOF {
        break
      }
      return err
    }

    // Write the buffer to the stream we opened earlier
    if err := stream.Send(&pb.PieceStore{Hash: hash, Size: length, Ttl: ttl, StoreOffset: storeOffset, Content: buffer[:n]}); err != nil {
  		return err
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

// Struct for reading piece download stream from server
type PieceStreamReader struct {
  stream pb.PieceStoreRoutes_RetrieveClient
}

// Read method for piece download stream
func (s *PieceStreamReader) Read(b []byte) (int, error) {
  pieceData, err := s.stream.Recv()
  if err != nil {
    return 0, err
  }

  n := copy(b, pieceData.Content)
  return n, err
}

// Begin Download Piece from Server
func RetrievePieceRequest(c pb.PieceStoreRoutesClient, hash string, offset int64, length int64 ) (io.Reader, error) {
  stream, err := c.Retrieve(context.Background(), &pb.PieceRetrieval{Hash: hash, Size: length, StoreOffset: offset})
  if err != nil {
    return nil, err
  }

  return &PieceStreamReader{stream: stream}, err
}

// Delete Piece From Server
func DeletePieceRequest(c pb.PieceStoreRoutesClient, hash string) (error) {
  reply, err := c.Delete(context.Background(), &pb.PieceDelete{Hash: hash})
  if err != nil {
    return err
  }
  log.Printf("Route summary : %v", reply)
  return nil
}
