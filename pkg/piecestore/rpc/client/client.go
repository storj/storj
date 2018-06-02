// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"fmt"
	"io"
	"log"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "storj.io/storj/protos/piecestore"
)

// Client -- Struct Info needed for protobuf api calls
type Client struct {
	ctx   context.Context
	route pb.PieceStoreRoutesClient
}

// New -- Initilize Client
func New(ctx context.Context, conn *grpc.ClientConn) *Client {
	return &Client{ctx, pb.NewPieceStoreRoutesClient(conn)}
}

// PieceMetaRequest -- Request info about a piece by Id
func (client *Client) PieceMetaRequest(id string) (*pb.PieceSummary, error) {
	return client.route.Piece(client.ctx, &pb.PieceId{Id: id})
}

// StorePieceRequest -- Upload Piece to Server
func (client *Client) StorePieceRequest(id string, dataOffset int64, length int64, ttl int64, storeOffset int64) (io.WriteCloser, error) {
	stream, err := client.route.Store(client.ctx)
	if err != nil {
		return nil, err
	}

	// SSend preliminary data
	if err := stream.Send(&pb.PieceStore{Id: id, Size: length, Ttl: ttl, StoreOffset: storeOffset}); err != nil {
		stream.CloseAndRecv()
		return nil, fmt.Errorf("%v.Send() = %v", stream, err)
	}

	return &StreamWriter{stream: stream}, err
}

// RetrievePieceRequest -- Begin Download Piece from Server
func (client *Client) RetrievePieceRequest(id string, offset int64, length int64) (io.ReadCloser, error) {
	stream, err := client.route.Retrieve(client.ctx, &pb.PieceRetrieval{Id: id, Size: length, StoreOffset: offset})
	if err != nil {
		return nil, err
	}

	return &StreamReader{stream: stream}, nil
}

// DeletePieceRequest -- Delete Piece From Server
func (client *Client) DeletePieceRequest(id string) error {
	reply, err := client.route.Delete(client.ctx, &pb.PieceDelete{Id: id})
	if err != nil {
		return err
	}
	log.Printf("Route summary : %v", reply)
	return nil
}
