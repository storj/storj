// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/ranger"
	pb "storj.io/storj/protos/piecestore"
)

// ClientError is any error returned by the client
var ClientError = errs.Class("PSClient error")

var (
	defaultBandwidthMsgSize = flag.Int(
		"piecestore.rpc.client.default_bandwidth_msg_size", 32*1024,
		"default bandwidth message size in kilobytes")
	maxBandwidthMsgSize = flag.Int(
		"piecestore.rpc.client.max_bandwidth_msg_size", 64*1024,
		"max bandwidth message size in kilobytes")
)

// PSClient is an interface describing the functions for interacting with piecestore nodes
type PSClient interface {
	Meta(ctx context.Context, id PieceID) (*pb.PieceSummary, error)
	Put(ctx context.Context, id PieceID, data io.Reader, ttl time.Time, ba *pb.PayerBandwidthAllocation) error
	Get(ctx context.Context, id PieceID, size int64, ba *pb.PayerBandwidthAllocation) (ranger.RangeCloser, error)
	Delete(ctx context.Context, pieceID PieceID) error
	io.Closer
}

// Client -- Struct Info needed for protobuf api calls
type Client struct {
	route            pb.PieceStoreRoutesClient
	conn             *grpc.ClientConn
	pkey             []byte
	bandwidthMsgSize int
}

// NewPSClient initilizes a PSClient
func NewPSClient(conn *grpc.ClientConn, bandwidthMsgSize int) (PSClient, error) {
	if bandwidthMsgSize < 0 || bandwidthMsgSize > *maxBandwidthMsgSize {
		return nil, ClientError.New(fmt.Sprintf("Invalid Bandwidth Message Size: %v", bandwidthMsgSize))
	}

	if bandwidthMsgSize == 0 {
		bandwidthMsgSize = *defaultBandwidthMsgSize
	}

	return &Client{
		conn:             conn,
		route:            pb.NewPieceStoreRoutesClient(conn),
		bandwidthMsgSize: bandwidthMsgSize,
	}, nil
}

// NewCustomRoute creates new Client with custom route interface
func NewCustomRoute(route pb.PieceStoreRoutesClient, bandwidthMsgSize int) (*Client, error) {
	if bandwidthMsgSize < 0 || bandwidthMsgSize > *maxBandwidthMsgSize {
		return nil, ClientError.New(fmt.Sprintf("Invalid Bandwidth Message Size: %v", bandwidthMsgSize))
	}

	if bandwidthMsgSize == 0 {
		bandwidthMsgSize = *defaultBandwidthMsgSize
	}

	return &Client{
		route:            route,
		bandwidthMsgSize: bandwidthMsgSize,
	}, nil
}

// Close closes the connection with piecestore
func (client *Client) Close() error {
	return client.conn.Close()
}

// Meta requests info about a piece by Id
func (client *Client) Meta(ctx context.Context, id PieceID) (*pb.PieceSummary, error) {
	return client.route.Piece(ctx, &pb.PieceId{Id: id.String()})
}

// Put uploads a Piece to a piece store Server
func (client *Client) Put(ctx context.Context, id PieceID, data io.Reader, ttl time.Time, ba *pb.PayerBandwidthAllocation) error {
	stream, err := client.route.Store(ctx)
	if err != nil {
		return err
	}

	msg := &pb.PieceStore{Piecedata: &pb.PieceStore_PieceData{Id: id.String(), ExpirationUnixSec: ttl.Unix()}}
	if err = stream.Send(msg); err != nil {
		if _, closeErr := stream.CloseAndRecv(); closeErr != nil {
			zap.S().Errorf("error closing stream %s :: %v.Send() = %v", closeErr, stream, closeErr)
		}

		return fmt.Errorf("%v.Send() = %v", stream, err)
	}

	writer := &StreamWriter{signer: client, stream: stream, pba: ba}

	defer func() {
		if err := writer.Close(); err != nil && err != io.EOF {
			log.Printf("failed to close writer: %s\n", err)
		}
	}()

	bufw := bufio.NewWriterSize(writer, 32*1024)

	_, err = io.Copy(bufw, data)
	if err == io.ErrUnexpectedEOF {
		writer.Close()
		zap.S().Infof("Node cut from upload due to slow connection. Deleting piece %s...", id)
		return client.Delete(ctx, id)
	}
	if err != nil {
		return err
	}

	return bufw.Flush()
}

// Get begins downloading a Piece from a piece store Server
func (client *Client) Get(ctx context.Context, id PieceID, size int64, ba *pb.PayerBandwidthAllocation) (ranger.RangeCloser, error) {
	stream, err := client.route.Retrieve(ctx)
	if err != nil {
		return nil, err
	}

	return PieceRangerSize(client, stream, id, size, ba), nil
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
