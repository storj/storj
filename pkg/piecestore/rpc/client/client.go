// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"fmt"
	"io"
	"log"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "storj.io/storj/protos/piecestore"
)

// PieceID - Id for piece
type PieceID string

// Client -- Struct Info needed for protobuf api calls
type Client struct {
	route pb.PieceStoreRoutesClient
}

// New -- Initilize Client
func NewPSClient(conn *grpc.ClientConn) *Client {
	return &Client{route: pb.NewPieceStoreRoutesClient(conn)}
}

// NewCustomRoute creates new Client with custom route interface
func NewCustomRoute(route pb.PieceStoreRoutesClient) *Client {
	return &Client{route: route}
}

// PieceMetaRequest -- Request info about a piece by Id
func (client *Client) PieceMetaRequest(ctx context.Context, id PieceID) (*pb.PieceSummary, error) {
	return client.route.Piece(ctx, &pb.PieceId{Id: id})
}

// Put -- Upload Piece to Server
func (client *Client) Put(ctx context.Context, id PieceID, ttl time.Time) (io.WriteCloser, error) {
	stream, err := client.route.Store(ctx)
	if err != nil {
		return nil, err
	}

	// SSend preliminary data
	if err := stream.Send(&pb.PieceStore{Id: id, Ttl: ttl}); err != nil {
		stream.CloseAndRecv()
		return nil, fmt.Errorf("%v.Send() = %v", stream, err)
	}

	return &StreamWriter{stream: stream}, err
}

// Get -- Begin Download Piece from Server
func (client *Client) Get(ctx context.Context, id PieceID, offset, length int64) (io.ReadCloser, error) {
	stream, err := client.route.Retrieve(ctx, &pb.PieceRetrieval{Id: id, Size: length, Offset: offset})
	if err != nil {
		return nil, err
	}

	return NewStreamReader(stream), nil
}

// Delete -- Delete Piece From Server
func (client *Client) Delete(ctx context.Context, id PieceID) error {
	reply, err := client.route.Delete(ctx, &pb.PieceDelete{Id: id})
	if err != nil {
		return err
	}
	log.Printf("Route summary : %v", reply)
	return nil
}
