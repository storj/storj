// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"io"
	"log"

	"github.com/zeebo/errs"

	"golang.org/x/net/context"

	pb "storj.io/storj/protos/piecestore"
)

var serverError = errs.Class("serverError")

// PieceMeta -- Struct containing Piece information from PieceMetaRequest
type PieceMeta struct {
	ID         string
	Size       int64
	Expiration int64
}

// PieceMetaRequest -- Request info about a piece by Id
func PieceMetaRequest(ctx context.Context, c pb.PieceStoreRoutesClient, id string) (*PieceMeta, error) {

	reply, err := c.Piece(ctx, &pb.PieceId{Id: id})
	if err != nil {
		return nil, err
	}

	return &PieceMeta{ID: reply.Id, Size: reply.Size, Expiration: reply.Expiration}, nil
}

// StorePieceRequest -- Upload Piece to Server
func StorePieceRequest(ctx context.Context, c pb.PieceStoreRoutesClient, id string, data io.Reader, dataOffset int64, length int64, ttl int64, storeOffset int64) error {
	buffer := make([]byte, 4096)

	stream, err := c.Store(ctx)
	if err != nil {
		return err
	}

	// SSend preliminary data
	if err := stream.Send(&pb.PieceStore{Id: id, Size: length, Ttl: ttl, StoreOffset: storeOffset}); err != nil {
		log.Printf("%v.Send() = %v", stream , err)
		goto endStorePiece
	}

	for {
		// Read data from read stream into buffer
		n, err := data.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("%v.Read() = %v", data , err)
			goto endStorePiece
		}

		// Write the buffer to the stream we opened earlier
		if err := stream.Send(&pb.PieceStore{Content: buffer[:n]}); err != nil {
			log.Printf("%v.Send() = %v", stream , err)
			goto endStorePiece
		}
	}

endStorePiece:
	reply, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}

	log.Printf("Route summary: %v", reply)

	return nil
}

// PieceStreamReader -- Struct for reading piece download stream from server
type PieceStreamReader struct {
	stream pb.PieceStoreRoutes_RetrieveClient
	overflowData []byte
}

// Read -- Read method for piece download stream
func (s *PieceStreamReader) Read(b []byte) (int, error) {
	if len(s.overflowData) > 0 {
		n := copy(b, s.overflowData)
		s.overflowData = s.overflowData[:len(s.overflowData) - n]
		return n, nil
	}

	pieceData, err := s.stream.Recv()
	if err != nil {
		return 0, err
	}

	n := copy(b, pieceData.Content)

	if cap(b) < len(pieceData.Content) {
		s.overflowData = b[cap(b):]
	}

	return n, nil
}

// Close -- Close Read Stream
func (s *PieceStreamReader) Close() error {
	return s.stream.CloseSend()
}

// RetrievePieceRequest -- Begin Download Piece from Server
func RetrievePieceRequest(ctx context.Context, c pb.PieceStoreRoutesClient, id string, offset int64, length int64) (io.ReadCloser, error) {
	stream, err := c.Retrieve(ctx, &pb.PieceRetrieval{Id: id, Size: length, StoreOffset: offset})
	if err != nil {
		return nil, err
	}

	return &PieceStreamReader{stream: stream}, err
}

// DeletePieceRequest -- Delete Piece From Server
func DeletePieceRequest(ctx context.Context, c pb.PieceStoreRoutesClient, id string) error {
	reply, err := c.Delete(ctx, &pb.PieceDelete{Id: id})
	if err != nil {
		return err
	}
	log.Printf("Route summary : %v", reply)
	return nil
}
