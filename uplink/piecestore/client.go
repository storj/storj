// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"io"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var Error = errs.Class("piecestore")

type Signer interface {
	ID() storj.NodeID
	HashAndSign(data []byte) ([]byte, error)
}

type Config struct {
	InitialStep int64
	MaximumStep int64
}

// Client can be used to implement psclient.Client

type Client struct {
	log *zap.Logger
	// TODO: hide
	signer Signer
	conn   *grpc.ClientConn
	client pb.PiecestoreClient
	config Config
}

func (client *Client) Delete(ctx context.Context, limit *pb.OrderLimit2) error {
	panic("TODO")
}

func (client *Client) Close() error {
	panic("TODO")
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
	next := previous * 3 / 2
	if next > client.config.MaximumStep {
		next = client.config.MaximumStep
	}
	return next
}
