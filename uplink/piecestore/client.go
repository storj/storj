// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"io"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
)

// Error is the default error class for piecestore client.
var Error = errs.Class("piecestore")

// Config defines piecestore client parameters fro upload and download.
type Config struct {
	UploadBufferSize   int64
	DownloadBufferSize int64

	InitialStep int64
	MaximumStep int64
}

// DefaultConfig are the default params used for upload and download.
var DefaultConfig = Config{
	UploadBufferSize:   256 * memory.KiB.Int64(),
	DownloadBufferSize: 256 * memory.KiB.Int64(),

	InitialStep: 64 * memory.KiB.Int64(),
	MaximumStep: 1 * memory.MiB.Int64(),
}

// Client implements uploading, downloading and deleting content from a piecestore.
type Client struct {
	log    *zap.Logger
	client rpc.PiecestoreClient
	conn   *rpc.Conn
	config Config
}

// Dial dials the target piecestore endpoint.
func Dial(ctx context.Context, dialer rpc.Dialer, target *pb.Node, log *zap.Logger, config Config) (*Client, error) {
	conn, err := dialer.DialNode(ctx, target)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &Client{
		log:    log,
		client: conn.PiecestoreClient(),
		conn:   conn,
		config: config,
	}, nil
}

// Delete uses delete order limit to delete a piece on piece store.
func (client *Client) Delete(ctx context.Context, limit *pb.OrderLimit, privateKey storj.PiecePrivateKey) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = client.client.Delete(ctx, &pb.PieceDeleteRequest{
		Limit: limit,
	})
	return Error.Wrap(err)
}

// Retain uses a bloom filter to tell the piece store which pieces to keep.
func (client *Client) Retain(ctx context.Context, req *pb.RetainRequest) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = client.client.Retain(ctx, req)
	return Error.Wrap(err)
}

// Close closes the underlying connection.
func (client *Client) Close() error {
	return client.conn.Close()
}

// next allocation step find the next trusted step.
func (client *Client) nextAllocationStep(previous int64) int64 {
	// TODO: ensure that this is frame idependent
	next := previous * 3 / 2
	if next > client.config.MaximumStep {
		next = client.config.MaximumStep
	}
	return next
}

// ignoreEOF is an utility func for ignoring EOF error, when it's not important.
func ignoreEOF(err error) error {
	if err == io.EOF {
		return nil
	}
	return err
}
