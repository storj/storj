// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package piecedeletion

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"

	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/overlay"
)

// Config defines configuration options for Service.
type Config struct {
	MaxConcurrency      int `help:"maximum number of concurrent requests to storage nodes" default:"100"`
	MaxConcurrentPieces int `help:"maximum number of concurrent pieces can be processed" default:"1000000" testDefault:"1000"`

	MaxPiecesPerBatch   int `help:"maximum number of pieces per batch" default:"5000" testDefault:"4000"`
	MaxPiecesPerRequest int `help:"maximum number pieces per single request" default:"1000" testDefault:"2000"`

	DialTimeout    time.Duration `help:"timeout for dialing nodes (0 means satellite default)" default:"3s" testDefault:"2s"`
	FailThreshold  time.Duration `help:"threshold for retrying a failed node" releaseDefault:"10m" devDefault:"2s"`
	RequestTimeout time.Duration `help:"timeout for a single delete request" releaseDefault:"15s" devDefault:"2s"`
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
	if config.MaxConcurrentPieces <= 0 {
		errlist.Add(Error.New("max concurrent pieces %d must be greater than 0", config.MaxConcurrentPieces))
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

// Nodes stores reliable nodes information.
type Nodes interface {
	GetNodes(ctx context.Context, nodes []storj.NodeID) (_ map[storj.NodeID]*overlay.SelectedNode, err error)
}

// Service handles combining piece deletion requests.
//
// architecture: Service
type Service struct {
	log    *zap.Logger
	config Config

	concurrentRequests *semaphore.Weighted

	rpcDialer rpc.Dialer
	nodesDB   Nodes

	running  sync2.Fence
	combiner *Combiner
	dialer   *Dialer
	limited  *LimitedHandler
}

// NewService creates a new service.
func NewService(log *zap.Logger, dialer rpc.Dialer, nodesDB Nodes, config Config) (*Service, error) {
	var errlist errs.Group
	if log == nil {
		errlist.Add(Error.New("log is nil"))
	}
	if dialer == (rpc.Dialer{}) {
		errlist.Add(Error.New("dialer is zero"))
	}
	if nodesDB == nil {
		errlist.Add(Error.New("nodesDB is nil"))
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

	if dialerClone.Pool == nil {
		dialerClone.Pool = rpc.NewDefaultConnectionPool()
	}

	return &Service{
		log:                log,
		config:             config,
		concurrentRequests: semaphore.NewWeighted(int64(config.MaxConcurrentPieces)),
		rpcDialer:          dialerClone,
		nodesDB:            nodesDB,
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
	if service.combiner != nil {
		service.combiner.Close()
	}
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

	// When number of pieces are more than the maximum limit, we let it overflow,
	// so we don't have to split requests in to separate batches.
	totalPieceCount := requestsPieceCount(requests)
	if totalPieceCount > service.config.MaxConcurrentPieces {
		totalPieceCount = service.config.MaxConcurrentPieces
	}

	if err := service.concurrentRequests.Acquire(ctx, int64(totalPieceCount)); err != nil {
		return Error.Wrap(err)
	}
	defer service.concurrentRequests.Release(int64(totalPieceCount))

	// Create a map for matching node information with the corresponding
	// request.
	nodesReqs := make(map[storj.NodeID]Request, len(requests))
	nodeIDs := []storj.NodeID{}
	for _, req := range requests {
		if req.Node.Address == "" {
			nodeIDs = append(nodeIDs, req.Node.ID)
		}
		nodesReqs[req.Node.ID] = req
	}

	if len(nodeIDs) > 0 {
		nodes, err := service.nodesDB.GetNodes(ctx, nodeIDs)
		if err != nil {
			// Pieces will be collected by garbage collector
			return Error.Wrap(err)
		}

		for _, node := range nodes {
			req := nodesReqs[node.ID]

			nodesReqs[node.ID] = Request{
				Node: storj.NodeURL{
					ID:      node.ID,
					Address: node.Address.Address,
				},
				Pieces: req.Pieces,
			}
		}
	}

	threshold, err := sync2.NewSuccessThreshold(len(nodesReqs), successThreshold)
	if err != nil {
		return Error.Wrap(err)
	}

	for _, req := range nodesReqs {
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
	Node   storj.NodeURL
	Pieces []storj.PieceID
}

// IsValid returns whether the request is valid.
func (req *Request) IsValid() bool {
	return !req.Node.ID.IsZero() && len(req.Pieces) > 0
}

func requestsPieceCount(requests []Request) int {
	total := 0
	for _, r := range requests {
		total += len(r.Pieces)
	}
	return total
}
