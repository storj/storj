// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"io"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcpool"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/uplink/private/piecestore"
)

// ErrNodeOffline is returned when it was not possible to contact a node or the node was not responding.
var ErrNodeOffline = errs.Class("node offline")

// VerifierConfig contains configurations for operation.
type VerifierConfig struct {
	DialTimeout        time.Duration `help:"how long to wait for a successful dial" default:"2s"`
	PerPieceTimeout    time.Duration `help:"duration to wait per piece download" default:"800ms"`
	OrderRetryThrottle time.Duration `help:"how much to wait before retrying order creation" default:"50ms"`

	RequestThrottle time.Duration `help:"minimum interval for sending out each request" default:"150ms"`
}

// NodeVerifier implements segment verification by dialing nodes.
type NodeVerifier struct {
	log *zap.Logger

	config VerifierConfig

	dialer rpc.Dialer
	orders *orders.Service
}

var _ Verifier = (*NodeVerifier)(nil)

// NewVerifier creates a new segment verifier using the specified dialer.
func NewVerifier(log *zap.Logger, dialer rpc.Dialer, orders *orders.Service, config VerifierConfig) *NodeVerifier {
	configuredDialer := dialer
	if config.DialTimeout > 0 {
		configuredDialer.DialTimeout = config.DialTimeout
	}

	configuredDialer.Pool = rpcpool.New(rpcpool.Options{
		Capacity:       1000,
		KeyCapacity:    5,
		IdleExpiration: 10 * time.Minute,
	})

	return &NodeVerifier{
		log:    log,
		config: config,
		dialer: configuredDialer,
		orders: orders,
	}
}

// Verify a collection of segments by attempting to download a byte from each segment from the target node.
func (service *NodeVerifier) Verify(ctx context.Context, alias metabase.NodeAlias, target storj.NodeURL, segments []*Segment, ignoreThrottle bool) error {
	client, err := piecestore.Dial(ctx, service.dialer, target, piecestore.DefaultConfig)
	if err != nil {
		return ErrNodeOffline.Wrap(err)
	}
	defer func() {
		if client != nil {
			_ = client.Close()
		}
	}()

	const maxRedials = 1
	redialCount := 0

	for i, segment := range segments {
		downloadStart := time.Now()
		err := service.verifySegment(ctx, client, alias, target, segment)

		if redialCount < maxRedials && ErrNodeOffline.Has(err) {
			redialCount++
			firstError := err

			_ = client.Close()
			client = nil
			if !sync2.Sleep(ctx, service.config.RequestThrottle) {
				return Error.Wrap(ctx.Err())
			}

			// redial the client
			client, err = piecestore.Dial(ctx, service.dialer, target, piecestore.DefaultConfig)
			if err != nil {
				return errs.Combine(ErrNodeOffline.Wrap(err), firstError)
			}

			// and then retry verifying the segment
			downloadStart = time.Now()
			err = service.verifySegment(ctx, client, alias, target, segment)
			if err != nil {
				return errs.Combine(Error.Wrap(err), firstError)
			}
		}

		if err != nil {
			return Error.Wrap(err)
		}
		throttle := service.config.RequestThrottle - time.Since(downloadStart)
		if !ignoreThrottle && throttle > 0 && i < len(segments)-1 {
			if !sync2.Sleep(ctx, throttle) {
				return Error.Wrap(ctx.Err())
			}
		}
	}
	return nil
}

// verifySegment tries to verify the segment by downloading a single byte from the specified segment.
func (service *NodeVerifier) verifySegment(ctx context.Context, client *piecestore.Client, alias metabase.NodeAlias, target storj.NodeURL, segment *Segment) error {
	pieceNum := findPieceNum(segment, alias)

	limit, piecePrivateKey, _, err := service.orders.CreateAuditOrderLimit(ctx, target.ID, pieceNum, segment.RootPieceID, segment.Redundancy.ShareSize)
	if err != nil {
		service.log.Error("failed to create order limit",
			zap.Stringer("retrying in", service.config.OrderRetryThrottle),
			zap.String("stream-id", segment.StreamID.String()),
			zap.Uint64("position", segment.Position.Encode()),
			zap.Error(err))

		if !sync2.Sleep(ctx, service.config.OrderRetryThrottle) {
			return Error.Wrap(ctx.Err())
		}

		limit, piecePrivateKey, _, err = service.orders.CreateAuditOrderLimit(ctx, target.ID, pieceNum, segment.RootPieceID, segment.Redundancy.ShareSize)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	timedCtx, cancel := context.WithTimeout(ctx, service.config.PerPieceTimeout)
	defer cancel()

	downloader, err := client.Download(timedCtx, limit.GetLimit(), piecePrivateKey, 0, 1)
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			service.log.Info("segment not found",
				zap.String("stream-id", segment.StreamID.String()),
				zap.Uint64("position", segment.Position.Encode()),
				zap.Error(err))
			segment.Status.MarkNotFound()
			return nil
		}

		service.log.Error("download failed",
			zap.String("stream-id", segment.StreamID.String()),
			zap.Uint64("position", segment.Position.Encode()),
			zap.Error(err))
		return ErrNodeOffline.Wrap(err)
	}

	buf := [1]byte{}
	_, errRead := io.ReadFull(downloader, buf[:])
	errClose := downloader.Close()

	err = errs.Combine(errClose, errRead)
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			service.log.Info("segment not found",
				zap.String("stream-id", segment.StreamID.String()),
				zap.Uint64("position", segment.Position.Encode()),
				zap.Error(err))
			segment.Status.MarkNotFound()
			return nil
		}

		service.log.Error("read/close failed",
			zap.String("stream-id", segment.StreamID.String()),
			zap.Uint64("position", segment.Position.Encode()),
			zap.Error(err))
		return ErrNodeOffline.Wrap(err)
	}
	segment.Status.MarkFound()

	return nil
}

func findPieceNum(segment *Segment, alias metabase.NodeAlias) uint16 {
	for _, p := range segment.AliasPieces {
		if p.Alias == alias {
			return p.Number
		}
	}
	panic("piece number not found")
}
