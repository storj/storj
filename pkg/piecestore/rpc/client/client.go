// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"errors"
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
	stream, err := c.Store(ctx)

	// SSend preliminary data
	if err := stream.Send(&pb.PieceStore{Id: id, Size: length, Ttl: ttl, StoreOffset: storeOffset}); err != nil {
		return err
	}

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
		if err := stream.Send(&pb.PieceStore{Content: buffer[:n]}); err != nil {
			return err
		}
	}

	reply, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}

	log.Printf("Route summary: %v", reply)

	if reply.Message != "OK" {
		return serverError.New(reply.Message)
	}

	return nil
}

// PieceStreamReader -- Struct for reading piece download stream from server
type PieceStreamReader struct {
	stream pb.PieceStoreRoutes_RetrieveClient
}

// Read -- Read method for piece download stream
func (s *PieceStreamReader) Read(b []byte) (int, error) {
	pieceData, err := s.stream.Recv()
	if err != nil {
		return 0, err
	}

	n := copy(b, pieceData.Content)

	if cap(b) < len(pieceData.Content) {
		return n, errors.New("Buffer wasn't large enough to read all data")
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
