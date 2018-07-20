// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"fmt"
	"io"
	"log"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/ranger"
	pb "storj.io/storj/protos/piecestore"
)

var defaultBandwidthMsgSize int = 32 * 1024
var maxBandwidthMsgSize int = 64 * 1024

// PSClient is an interface describing the functions for interacting with piecestore nodes
type PSClient interface {
	Meta(ctx context.Context, id PieceID) (*pb.PieceSummary, error)
	Put(ctx context.Context, id PieceID, data io.Reader, ttl time.Time) error
	Get(ctx context.Context, id PieceID, size int64) (ranger.RangeCloser, error)
	Delete(ctx context.Context, pieceID PieceID) error
	CloseConn() error
}

// Client -- Struct Info needed for protobuf api calls
type Client struct {
	route            pb.PieceStoreRoutesClient
	conn             *grpc.ClientConn
	pkey             []byte
	bandwidthMsgSize int
	payerID          string
	renterID         string
}

// NewPSClient initilizes a PSClient
func NewPSClient(conn *grpc.ClientConn, bandwidthMsgSize int, payerID, renterID string) PSClient {
	if bandwidthMsgSize < 0 || bandwidthMsgSize > maxBandwidthMsgSize {
		log.Printf("Invalid Bandwidth Message Size: %v", bandwidthMsgSize)

		return nil
	}

	if bandwidthMsgSize == 0 {
		bandwidthMsgSize = defaultBandwidthMsgSize
	}

	return &Client{
		conn:    conn,
		route:   pb.NewPieceStoreRoutesClient(conn),
		payerID: payerID, renterID: renterID,
		bandwidthMsgSize: bandwidthMsgSize,
	}
}

// NewCustomRoute creates new Client with custom route interface
func NewCustomRoute(route pb.PieceStoreRoutesClient, bandwidthMsgSize int, payerID, renterID string) *Client {
	if bandwidthMsgSize < 0 || bandwidthMsgSize > maxBandwidthMsgSize {
		log.Printf("Invalid Bandwidth Message Size: %v", bandwidthMsgSize)

		return nil
	}

	if bandwidthMsgSize == 0 {
		bandwidthMsgSize = defaultBandwidthMsgSize
	}

	return &Client{route: route, payerID: payerID, renterID: renterID, bandwidthMsgSize: bandwidthMsgSize}
}

// CloseConn closes the connection with piecestore
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
	msg := &pb.PieceStore{Piecedata: &pb.PieceStore_PieceData{Id: id.String(), Ttl: ttl.Unix()}}
	if err = stream.Send(msg); err != nil {
		if _, closeErr := stream.CloseAndRecv(); closeErr != nil {
			zap.S().Errorf("error closing stream %s :: %v.Send() = %v", closeErr, stream, closeErr)
		}

		return fmt.Errorf("%v.Send() = %v", stream, err)
	}

	writer := &StreamWriter{signer: client, stream: stream}

	defer func() {
		if err = writer.Close(); err != nil {
			log.Printf("failed to close writer: %s\n", err)
		}
	}()

	_, err = io.Copy(writer, data)

	return err
}

// Get begins downloading a Piece from a piece store Server
func (client *Client) Get(ctx context.Context, id PieceID, size int64) (ranger.RangeCloser, error) {
	stream, err := client.route.Retrieve(ctx)
	if err != nil {
		return nil, err
	}

	return PieceRangerSize(client, stream, id, size), nil
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

// sign a message using the clients private key
func (client *Client) sign(msg []byte) (signature []byte, err error) {
	// use c.pkey to sign msg

	return signature, err
}
