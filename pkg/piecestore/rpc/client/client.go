// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/mr-tron/base58/base58"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/ranger"
	pb "storj.io/storj/protos/piecestore"
)

type PSClient interface {
	Meta(ctx context.Context, id PieceID) (*pb.PieceSummary, error)
	Put(ctx context.Context, id PieceID, data io.Reader, ttl time.Time) error
	Get(ctx context.Context, id PieceID, size int64) (ranger.RangeCloser, error)
	Delete(ctx context.Context, pieceID PieceID) error
	CloseConn() error
}

// PieceID - Id for piece
type PieceID string

// String -- Get String from PieceID
func (id PieceID) String() string {
	return string(id)
}

// Client -- Struct Info needed for protobuf api calls
type Client struct {
	route pb.PieceStoreRoutesClient
	conn  *grpc.ClientConn
}

// NewPSClient initilizes a PSClient
func NewPSClient(conn *grpc.ClientConn) PSClient {
	return &Client{conn: conn, route: pb.NewPieceStoreRoutesClient(conn)}
}

// NewCustomRoute creates new Client with custom route interface
func NewCustomRoute(route pb.PieceStoreRoutesClient) *Client {
	return &Client{route: route}
}

func (client *Client) CloseConn() error {
	return client.conn.Close()
}

// Meta requests info about a piece by Id
func (client *Client) Meta(ctx context.Context, id PieceID) (*pb.PieceSummary, error) {
	return client.route.Piece(ctx, &pb.PieceId{Id: id.String()})
}

// Put uploads a Piece to a piece store Server
func (client *Client) Put(ctx context.Context, id PieceID, data io.Reader, ttl time.Time) error {
	stream, err := client.route.Store(ctx)
	if err != nil {
		return err
	}

	// Send preliminary data
	if err := stream.Send(&pb.PieceStore{Id: id.String(), Ttl: ttl.Unix()}); err != nil {
		stream.CloseAndRecv()
		return fmt.Errorf("%v.Send() = %v", stream, err)
	}

	writer := &StreamWriter{stream: stream}

	defer writer.Close()

	_, err = io.Copy(writer, data)

	return err
}

// Get begins downloading a Piece from a piece store Server
func (client *Client) Get(ctx context.Context, id PieceID, size int64) (ranger.RangeCloser, error) {
	return PieceRangerSize(client, id, size), nil
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

func (id PieceID) IsValid() bool {
	return len(id) >= 20
}

// NewPieceID creates a PieceID
func NewPieceID() PieceID {
	b := make([]byte, 32)

	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	return PieceID(base58.Encode(b))
}
