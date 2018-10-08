// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"bufio"
	"crypto"
	"crypto/ecdsa"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/gtank/cryptopasta"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/ranger"
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
	Get(ctx context.Context, id PieceID, size int64, ba *pb.PayerBandwidthAllocation) (ranger.Ranger, error)
	Delete(ctx context.Context, pieceID PieceID) error
	Stats(ctx context.Context) (*pb.StatSummary, error)
	io.Closer
}

// Client -- Struct Info needed for protobuf api calls
type Client struct {
	route            pb.PieceStoreRoutesClient
	conn             *grpc.ClientConn
	prikey           crypto.PrivateKey
	bandwidthMsgSize int
}

// NewPSClient initilizes a PSClient
func NewPSClient(conn *grpc.ClientConn, bandwidthMsgSize int, prikey crypto.PrivateKey) (PSClient, error) {
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
		prikey:           prikey,
	}, nil
}

// NewCustomRoute creates new Client with custom route interface
func NewCustomRoute(route pb.PieceStoreRoutesClient, bandwidthMsgSize int, prikey crypto.PrivateKey) (*Client, error) {
	if bandwidthMsgSize < 0 || bandwidthMsgSize > *maxBandwidthMsgSize {
		return nil, ClientError.New(fmt.Sprintf("Invalid Bandwidth Message Size: %v", bandwidthMsgSize))
	}

	if bandwidthMsgSize == 0 {
		bandwidthMsgSize = *defaultBandwidthMsgSize
	}

	return &Client{
		route:            route,
		bandwidthMsgSize: bandwidthMsgSize,
		prikey:           prikey,
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
		_ = writer.Close()
		zap.S().Infof("Node cut from upload due to slow connection. Deleting piece %s...", id)
		deleteErr := client.Delete(ctx, id)
		if deleteErr != nil {
			return deleteErr
		}
	}
	if err != nil {
		return err
	}

	return bufw.Flush()
}

// Get begins downloading a Piece from a piece store Server
func (client *Client) Get(ctx context.Context, id PieceID, size int64, ba *pb.PayerBandwidthAllocation) (ranger.Ranger, error) {
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

// Stats will retrieve stats about a piece storage node
func (client *Client) Stats(ctx context.Context) (*pb.StatSummary, error) {
	return client.route.Stats(ctx, &pb.StatsReq{})
}

// sign a message using the clients private key
func (client *Client) sign(msg []byte) (signature []byte, err error) {
	if client.prikey == nil {
		return nil, ClientError.New("Failed to sign msg: Private Key not Set")
	}

	// use c.pkey to sign msg
	return cryptopasta.Sign(msg, client.prikey.(*ecdsa.PrivateKey))
}
