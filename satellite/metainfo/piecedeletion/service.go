// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package piecedeletion

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/sync2"
)

// Config defines configuration options for Service.
type Config struct {
	MaxConcurrency int `help:"maximum number of concurrent requests to storage nodes" default:"100"`

	MaxPiecesPerBatch   int `help:"maximum number of pieces per batch" default:"5000"`
	MaxPiecesPerRequest int `help:"maximum number pieces per single request" default:"1000"`

	DialTimeout    time.Duration `help:"timeout for dialing nodes (0 means satellite default)" default:"0"`
	FailThreshold  time.Duration `help:"threshold for retrying a failed node" releaseDefault:"5m" devDefault:"2s"`
	RequestTimeout time.Duration `help:"timeout for a single delete request" releaseDefault:"1m" devDefault:"2s"`
}

const (
	minTimeout = 5 * time.Millisecond
	maxTimeout = 5 * time.Minute
)

// Verify verifies configuration sanity.
func (config *Config) Verify() errs.Group {
	var errlist errs.Group
	if config.MaxConcurrency <= 0 {
		errlist.Add(Error.New("concurrency %d must be greater than 0", config.MaxConcurrency))
	}
	if config.MaxPiecesPerBatch < config.MaxPiecesPerRequest {
		errlist.Add(Error.New("max pieces per batch %d should be larger than max pieces per request %d", config.MaxPiecesPerBatch, config.MaxPiecesPerRequest))
	}
	if config.MaxPiecesPerBatch <= 0 {
		errlist.Add(Error.New("max pieces per batch %d must be greater than 0", config.MaxPiecesPerBatch))
	}
	if config.MaxPiecesPerRequest <= 0 {
		errlist.Add(Error.New("max pieces per request %d must be greater than 0", config.MaxPiecesPerRequest))
	}
	if config.DialTimeout != 0 && (config.DialTimeout <= minTimeout || maxTimeout <= config.DialTimeout) {
		errlist.Add(Error.New("dial timeout %v must be between %v and %v", config.DialTimeout, minTimeout, maxTimeout))
	}
	if config.RequestTimeout < minTimeout || maxTimeout < config.RequestTimeout {
		errlist.Add(Error.New("request timeout %v should be between %v and %v", config.RequestTimeout, minTimeout, maxTimeout))
	}
	return errlist
}

// Service handles combining piece deletion requests.
//
// architecture: Service
type Service struct {
	log    *zap.Logger
	config Config

	rpcDialer rpc.Dialer

	running  sync2.Fence
	combiner *Combiner
	dialer   *Dialer
	limited  *LimitedHandler
}

// NewService creates a new service.
func NewService(log *zap.Logger, dialer rpc.Dialer, config Config) (*Service, error) {
	var errlist errs.Group
	if log == nil {
		errlist.Add(Error.New("log is nil"))
	}
	if dialer == (rpc.Dialer{}) {
		errlist.Add(Error.New("dialer is zero"))
	}
	if errs := config.Verify(); len(errs) > 0 {
		errlist.Add(errs...)
	}
	if err := errlist.Err(); err != nil {
		return nil, Error.Wrap(err)
	}

	dialerClone := dialer
	if config.DialTimeout > 0 {
		dialerClone.DialTimeout = config.DialTimeout
	}

	return &Service{
		log:       log,
		config:    config,
		rpcDialer: dialerClone,
	}, nil
}

// newQueue creates the configured queue.
func (service *Service) newQueue() Queue {
	return NewLimitedJobs(service.config.MaxPiecesPerBatch)
}

// Run initializes the service.
func (service *Service) Run(ctx context.Context) error {
	defer service.running.Release()

	config := service.config
	service.dialer = NewDialer(service.log.Named("dialer"), service.rpcDialer, config.RequestTimeout, config.FailThreshold, config.MaxPiecesPerRequest)
	service.limited = NewLimitedHandler(service.dialer, config.MaxConcurrency)
	service.combiner = NewCombiner(ctx, service.limited, service.newQueue)

	return nil
}

// Close shuts down the service.
func (service *Service) Close() error {
	<-service.running.Done()
	service.combiner.Close()
	return nil
}

// Delete deletes the pieces specified in the requests waiting until success threshold is reached.
func (service *Service) Delete(ctx context.Context, requests []Request, successThreshold float64) (err error) {
	defer mon.Task()(&ctx, len(requests), requestsPieceCount(requests), successThreshold)(&err)

	if len(requests) == 0 {
		return nil
	}

	// wait for combiner and dialer to set themselves up.
	if !service.running.Wait(ctx) {
		return Error.Wrap(ctx.Err())
	}

	for i, req := range requests {
		if !req.IsValid() {
			return Error.New("request #%d is invalid", i)
		}
	}

	threshold, err := sync2.NewSuccessThreshold(len(requests), successThreshold)
	if err != nil {
		return Error.Wrap(err)
	}

	for _, req := range requests {
		service.combiner.Enqueue(req.Node, Job{
			Pieces:  req.Pieces,
			Resolve: threshold,
		})
	}

	threshold.Wait(ctx)

	return nil
}

// Request defines a deletion requests for a node.
type Request struct {
	Node   *pb.Node
	Pieces []storj.PieceID
}

// IsValid returns whether the request is valid.
func (req *Request) IsValid() bool {
	return req.Node != nil &&
		!req.Node.Id.IsZero() &&
		len(req.Pieces) > 0
}

func requestsPieceCount(requests []Request) int {
	total := 0
	for _, r := range requests {
		total += len(r.Pieces)
	}
	return total
}
