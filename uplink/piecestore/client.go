// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"io"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/pb"
)

var Error = errs.Class("piecestore")

type Config struct {
	InitialStep int64
	MaximumStep int64
}

var DefaultConfig = Config{
	InitialStep: 256 * memory.KiB.Int64(),
	MaximumStep: 5 * memory.MiB.Int64(),
}

type Client struct {
	log    *zap.Logger
	signer signing.Signer
	conn   *grpc.ClientConn
	client pb.PiecestoreClient
	config Config
}

func NewClient(log *zap.Logger, signer signing.Signer, conn *grpc.ClientConn, config Config) *Client {
	return &Client{
		log:    log,
		signer: signer,
		conn:   conn,
		client: pb.NewPiecestoreClient(conn),
		config: config,
	}
}

func (client *Client) Delete(ctx context.Context, limit *pb.OrderLimit2) error {
	_, err := client.client.Delete(ctx, &pb.PieceDeleteRequest{
		Limit: limit,
	})
	return Error.Wrap(err)
}

func (client *Client) Close() error {
	return client.conn.Close()
}

func combineSendCloseError(sendError, closeError error) error {
	if sendError != nil && closeError != nil {
		if sendError == io.EOF {
			sendError = nil
		}
	}
	return errs.Combine(closeError, sendError)
}

func (client *Client) nextAllocationStep(previous int64) int64 {
	// TODO: ensure that this is frame idependent
	next := previous * 3 / 2
	if next > client.config.MaximumStep {
		next = client.config.MaximumStep
	}
	return next
}
