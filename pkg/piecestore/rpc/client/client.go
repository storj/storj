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

	"storj.io/storj/pkg/ranger"
	pb "storj.io/storj/protos/piecestore"
)

// PSClient is an interface describing the functions for interacting with piecestore nodes
type PSClient interface {
	Meta(ctx context.Context, id PieceID) (*pb.PieceSummary, error)
	Put(ctx context.Context, id PieceID, data io.Reader, ttl time.Time, payer, bandwidthClient string) error
	Get(ctx context.Context, id PieceID, size int64, payer, bandwidthClient string) (ranger.RangeCloser, error)
	Delete(ctx context.Context, pieceID PieceID) error
	Sign(ctx context.Context, message []byte) (signature []byte, err error)
	CloseConn() error
}

// Client -- Struct Info needed for protobuf api calls
type Client struct {
	route pb.PieceStoreRoutesClient
	conn  *grpc.ClientConn
	pkey  []byte
}

// NewPSClient initilizes a PSClient
func NewPSClient(conn *grpc.ClientConn) PSClient {
	return &Client{conn: conn, route: pb.NewPieceStoreRoutesClient(conn)}
}

// NewCustomRoute creates new Client with custom route interface
func NewCustomRoute(route pb.PieceStoreRoutesClient) *Client {
	return &Client{route: route}
}

// CloseConn closes the connection with piecestore
func (client *Client) CloseConn() error {
	return client.conn.Close()
}

// Sign a message using the clients private key
func (client *Client) Sign(ctx context.Context, msg []byte) (signature []byte, err error) {
	// use c.pkey to sign msg

	return signature, err
}

// Meta requests info about a piece by Id
func (client *Client) Meta(ctx context.Context, id PieceID) (*pb.PieceSummary, error) {
	return client.route.Piece(ctx, &pb.PieceId{Id: id.String()})
}

// Put uploads a Piece to a piece store Server
func (client *Client) Put(ctx context.Context, id PieceID, data io.Reader, ttl time.Time, payer, bandwidthClient string) error {
	stream, err := client.route.Store(ctx)
	if err != nil {
		return err
	}

	// Send preliminary data
	if err := stream.Send(&pb.PieceStore{Piecedata: &pb.PieceStore_PieceData{Id: id.String(), Ttl: ttl.Unix()}}); err != nil {
		stream.CloseAndRecv()
		return fmt.Errorf("%v.Send() = %v", stream, err)
	}

	writer := &StreamWriter{signer: client, payer: payer, bandwidthClient: bandwidthClient, stream: stream}

	defer writer.Close()

	_, err = io.Copy(writer, data)

	return err
}

// Get begins downloading a Piece from a piece store Server
func (client *Client) Get(ctx context.Context, id PieceID, size int64, payer, bandwidthClient string) (ranger.RangeCloser, error) {
	stream, err := client.route.Retrieve(ctx)
	if err != nil {
		return nil, err
	}

	return PieceRangerSize(client, stream, id, size, payer, bandwidthClient), nil
}

// Delete a Piece from a piece store Server
func (client *Client) Delete(ctx context.Context, id PieceID) error {
	reply, err := client.route.Delete(ctx, &pb.PieceDelete{Id: id.String()})
	if err != nil {
		return err
	}
	log.Printf("Route summary : %v", reply)
	return nil
}
