// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/blang/semver"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcpool"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/uplink/private/piecestore"
)

// ErrNodeOffline is returned when it was not possible to contact a node or the node was not responding.
var ErrNodeOffline = errs.Class("node offline")

// ErrNoSuchNode is returned when a node has been disqualified or gracefully exited, or no known node
// has the specified alias. Pieces associated with the given alias should be considered lost.
var ErrNoSuchNode = errs.Class("no such node")

var errWrongNodeVersion = errs.Class("wrong node version")

// VerifierConfig contains configurations for operation.
type VerifierConfig struct {
	DialTimeout        time.Duration `help:"how long to wait for a successful dial" default:"2s"`
	PerPieceTimeout    time.Duration `help:"duration to wait per piece download" default:"800ms"`
	OrderRetryThrottle time.Duration `help:"how much to wait before retrying order creation" default:"50ms"`

	RequestThrottle   time.Duration `help:"minimum interval for sending out each request" default:"150ms"`
	VersionWithExists string        `help:"minimum storage node version with implemented Exists method" default:"v1.69.2"`
}

// NodeVerifier implements segment verification by dialing nodes.
type NodeVerifier struct {
	log *zap.Logger

	config VerifierConfig

	dialer rpc.Dialer
	orders *orders.Service

	// this is a callback so that problematic pieces can be reported as they are found,
	// rather than being kept in a list which might grow unreasonably large.
	reportPiece pieceReporterFunc

	versionWithExists semver.Version
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

	version, err := semver.ParseTolerant(config.VersionWithExists)
	if err != nil {
		log.Warn("invalid VersionWithExists", zap.String("VersionWithExists", config.VersionWithExists), zap.Error(err))
	}

	return &NodeVerifier{
		log:               log,
		config:            config,
		dialer:            configuredDialer,
		orders:            orders,
		versionWithExists: version,
	}
}

// Verify a collection of segments by attempting to download a byte from each segment from the target node.
func (service *NodeVerifier) Verify(ctx context.Context, alias metabase.NodeAlias, target storj.NodeURL, targetVersion string, segments []*Segment, ignoreThrottle bool) (verifiedCount int, err error) {
	verifiedCount, err = service.VerifyWithExists(ctx, alias, target, targetVersion, segments)
	// if Exists method is unimplemented or it is wrong node version fallback to download verification
	if !methodUnimplemented(err) && !errWrongNodeVersion.Has(err) {
		return verifiedCount, err
	}
	if err != nil {
		service.log.Debug("fallback to download method", zap.Error(err))
	}

	service.log.Debug("verify segments by downloading pieces")

	var client *piecestore.Client
	defer func() {
		if client != nil {
			_ = client.Close()
		}
	}()

	const maxDials = 2
	dialCount := 0

	rateLimiter := newRateLimiter(0, 0)
	if !ignoreThrottle {
		rateLimiter = newRateLimiter(service.config.RequestThrottle, service.config.RequestThrottle/4)
	}

	nextRequest := time.Now()
	for i, segment := range segments {
		nextRequest, err = rateLimiter.next(ctx, nextRequest)
		if err != nil {
			return i, Error.Wrap(err)
		}

		for client == nil {
			dialCount++
			if dialCount > maxDials {
				return i, ErrNodeOffline.New("too many redials")
			}
			client, err = piecestore.Dial(rpcpool.WithForceDial(ctx), service.dialer, target, piecestore.DefaultConfig)
			if err != nil {
				service.log.Info("failed to dial node",
					zap.Stringer("node-id", target.ID),
					zap.Error(err))
				client = nil
				nextRequest, err = rateLimiter.next(ctx, nextRequest)
				if err != nil {
					return i, Error.Wrap(err)
				}
			}
		}

		outcome, err := service.verifySegment(ctx, client, alias, target, segment)
		if err != nil {
			// we could not do the verification, for a reason that implies we won't be able
			// to do any more
			return i, Error.Wrap(err)
		}
		switch outcome {
		case audit.OutcomeNodeOffline, audit.OutcomeTimedOut:
			_ = client.Close()
			client = nil
		case audit.OutcomeFailure:
			segment.Status.MarkNotFound()
		case audit.OutcomeSuccess:
			segment.Status.MarkFound()
		}
	}
	return len(segments), nil
}

// verifySegment tries to verify the segment by downloading a single byte from the piece of the segment
// on the specified target node.
func (service *NodeVerifier) verifySegment(ctx context.Context, client *piecestore.Client, alias metabase.NodeAlias, target storj.NodeURL, segment *Segment) (outcome audit.Outcome, err error) {
	pieceNum := findPieceNum(segment, alias)

	logger := service.log.With(
		zap.Stringer("stream-id", segment.StreamID),
		zap.Stringer("node-id", target.ID),
		zap.Uint64("position", segment.Position.Encode()),
		zap.Uint16("piece-num", pieceNum))

	defer func() {
		// report the outcome of the piece check, if required
		if outcome != audit.OutcomeSuccess && service.reportPiece != nil {
			reportErr := service.reportPiece(ctx, &segment.VerifySegment, target.ID, int(pieceNum), outcome)
			err = errs.Combine(err, reportErr)
		}
	}()

	limit, piecePrivateKey, _, err := service.orders.CreateAuditOrderLimit(ctx, target.ID, pieceNum, segment.RootPieceID, segment.Redundancy.ShareSize)
	if err != nil {
		logger.Error("failed to create order limit",
			zap.Stringer("retrying in", service.config.OrderRetryThrottle),
			zap.Error(err))

		if !sync2.Sleep(ctx, service.config.OrderRetryThrottle) {
			return audit.OutcomeNotPerformed, Error.Wrap(ctx.Err())
		}

		limit, piecePrivateKey, _, err = service.orders.CreateAuditOrderLimit(ctx, target.ID, pieceNum, segment.RootPieceID, segment.Redundancy.ShareSize)
		if err != nil {
			return audit.OutcomeNotPerformed, Error.Wrap(err)
		}
	}

	timedCtx, cancel := context.WithTimeout(ctx, service.config.PerPieceTimeout)
	defer cancel()

	downloader, err := client.Download(timedCtx, limit.GetLimit(), piecePrivateKey, 0, 1)
	if err != nil {
		logger.Error("download failed", zap.Error(err))
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			return audit.OutcomeFailure, nil
		}
		if errs2.IsRPC(err, rpcstatus.Unknown) {
			// dial failed -- offline node
			return audit.OutcomeNodeOffline, nil
		}
		return audit.OutcomeUnknownError, nil
	}

	buf := [1]byte{}
	_, errRead := io.ReadFull(downloader, buf[:])
	errClose := downloader.Close()

	err = errs.Combine(errClose, errRead)
	if err != nil {
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			logger.Info("segment not found", zap.Error(err))
			return audit.OutcomeFailure, nil
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			logger.Info("failed to get timely response when asking for piece", zap.Error(err))
			return audit.OutcomeTimedOut, nil
		}

		logger.Error("read/close failed", zap.Error(err))
		return audit.OutcomeUnknownError, nil
	}

	logger.Info("download succeeded")
	return audit.OutcomeSuccess, nil
}

func findPieceNum(segment *Segment, alias metabase.NodeAlias) uint16 {
	for _, p := range segment.AliasPieces {
		if p.Alias == alias {
			return p.Number
		}
	}
	panic("piece number not found")
}

// VerifyWithExists verifies that the segments exist on the specified node by calling the piecestore Exists
// endpoint if the node version supports it.
func (service *NodeVerifier) VerifyWithExists(ctx context.Context, alias metabase.NodeAlias, target storj.NodeURL, targetVersion string, segments []*Segment) (verifiedCount int, err error) {
	defer mon.Task()(&ctx)(&err)

	if service.versionWithExists.String() == "" || targetVersion == "" {
		return 0, errWrongNodeVersion.New("missing node version or no base version defined")
	}

	nodeVersion, err := semver.ParseTolerant(targetVersion)
	if err != nil {
		return 0, errWrongNodeVersion.Wrap(err)
	}

	if !nodeVersion.GE(service.versionWithExists) {
		return 0, errWrongNodeVersion.New("too old version")
	}

	service.log.Debug("verify segments using Exists method", zap.Stringer("node-id", target.ID))

	var conn *rpc.Conn
	var client pb.DRPCPiecestoreClient
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()

	const maxDials = 2
	dialCount := 0

	for client == nil {
		dialCount++
		if dialCount > maxDials {
			return 0, ErrNodeOffline.New("too many redials")
		}

		conn, err := service.dialer.DialNodeURL(rpcpool.WithForceDial(ctx), target)
		if err != nil {
			service.log.Info("failed to dial node",
				zap.Stringer("node-id", target.ID),
				zap.Error(err))
		} else {
			client = pb.NewDRPCPiecestoreClient(conn)
		}
	}

	// this is maybe too generous, since Exists should be faster than asking for pieces one by one,
	// but seems like a good first try
	ctx, cancel := context.WithTimeout(ctx, service.config.PerPieceTimeout*time.Duration(len(segments)))
	defer cancel()

	err = service.verifySegmentsWithExists(ctx, client, alias, target, segments)
	if err != nil {
		// we could not do the verification, for a reason that implies we won't be able
		// to do any more
		return 0, Error.Wrap(err)
	}

	return len(segments), nil
}

func methodUnimplemented(err error) bool {
	return errs2.IsRPC(err, rpcstatus.Unimplemented) || errs2.IsRPC(err, rpcstatus.Unknown)
}

// verifySegmentsWithExists calls the Exists endpoint on the specified target node for each segment.
func (service *NodeVerifier) verifySegmentsWithExists(ctx context.Context, client pb.DRPCPiecestoreClient, alias metabase.NodeAlias, target storj.NodeURL, segments []*Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	pieceIds := make([]storj.PieceID, 0, len(segments))
	pieceNums := make([]uint16, 0, len(segments))

	for _, segment := range segments {
		pieceNum := findPieceNum(segment, alias)
		pieceNums = append(pieceNums, pieceNum)
		pieceId := segment.RootPieceID.Derive(target.ID, int32(pieceNum))
		pieceIds = append(pieceIds, pieceId)
	}

	response, err := client.Exists(ctx, &pb.ExistsRequest{
		PieceIds: pieceIds,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	service.log.Debug("Exists response",
		zap.Int("request-count", len(pieceIds)),
		zap.Int("missing-count", len(response.Missing)))

	if len(response.Missing) == 0 {
		for i := range segments {
			segments[i].Status.MarkFound()
		}
		return nil
	}

	missing := make(map[uint32]struct{}, len(segments))
	for _, m := range response.Missing {
		missing[m] = struct{}{}
	}

	for i := range segments {
		_, miss := missing[uint32(i)]
		if miss {
			segments[i].Status.MarkNotFound()
			if service.reportPiece != nil {
				reportErr := service.reportPiece(ctx, &segments[i].VerifySegment, target.ID, int(pieceNums[i]), audit.OutcomeFailure)
				err = errs.Combine(err, reportErr)
			}
		} else {
			segments[i].Status.MarkFound()
		}
	}

	return err
}

// rateLimiter limits the rate of some type of event. It acts like a token
// bucket, allowing for bursting, as long as the _average_ interval between
// events over the lifetime of the rateLimiter is less than or equal to the
// specified averageInterval.
//
// The wait time between events will be at least minInterval, even if an
// event would otherwise have been allowed with bursting.
type rateLimiter struct {
	nowFn           func() time.Time
	sleepFn         func(ctx context.Context, t time.Duration) bool
	averageInterval time.Duration
	minInterval     time.Duration
}

// newRateLimiter creates a new rateLimiter. If both arguments are specified
// as 0, the rate limiter will have no effect (next() will always return
// immediately).
func newRateLimiter(averageInterval, minInterval time.Duration) rateLimiter {
	return rateLimiter{
		nowFn:           time.Now,
		sleepFn:         sync2.Sleep,
		averageInterval: averageInterval,
		minInterval:     minInterval,
	}
}

// next() sleeps until the time when the next event should be allowed.
// It should be passed the current time for the first call. After that,
// the value returned by the last call to next() should be passed in
// as its argument.
func (r rateLimiter) next(ctx context.Context, nextTime time.Time) (time.Time, error) {
	sleepTime := nextTime.Sub(r.nowFn())
	if sleepTime < r.minInterval {
		sleepTime = r.minInterval
	}
	if !r.sleepFn(ctx, sleepTime) {
		return time.Time{}, ctx.Err()
	}
	return nextTime.Add(r.averageInterval), nil
}
