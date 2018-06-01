// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"fmt"
	"io"
	"log"

	"golang.org/x/net/context"

	pb "storj.io/storj/protos/piecestore"
)

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
func StorePieceRequest(ctx context.Context, c pb.PieceStoreRoutesClient, id string, dataOffset int64, length int64, ttl int64, storeOffset int64) (io.WriteCloser, error) {
	stream, err := c.Store(ctx)
	if err != nil {
		return nil, err
	}

	// SSend preliminary data
	if err := stream.Send(&pb.PieceStore{Id: id, Size: length, Ttl: ttl, StoreOffset: storeOffset}); err != nil {
		stream.CloseAndRecv()
		return nil, fmt.Errorf("%v.Send() = %v", stream, err)
	}

	return &ClientStreamWriter{stream: stream}, err
}

// RetrievePieceRequest -- Begin Download Piece from Server
func RetrievePieceRequest(ctx context.Context, c pb.PieceStoreRoutesClient, id string, offset int64, length int64) (io.ReadCloser, error) {
	stream, err := c.Retrieve(ctx, &pb.PieceRetrieval{Id: id, Size: length, StoreOffset: offset})
	if err != nil {
		return nil, err
	}

	return &ClientStreamReader{stream: stream}, nil
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
