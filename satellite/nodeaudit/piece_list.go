// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeaudit

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/taskqueue"
)

// PieceListConfig holds the observer configuration.
type PieceListConfig struct {
	Node string `required:"true" help:"Node ID of the node to find pieces for."`

	nodeID storj.NodeID
}

// Job represents a node audit task in the queue.
type Job struct {
	StreamID    uuid.UUID `redis:"stream_id"`
	Position    uint64    `redis:"position"`
	PieceNo     uint16
	RootPieceID storj.PieceID
}

// PieceList implements rangedloop.PieceList.
// It finds all segments with pieces on a specific node and pushes them to the task queue.
type PieceList struct {
	config PieceListConfig
	client *taskqueue.Client
}

// NewPieceList creates a new node audit observer.
func NewPieceList(client *taskqueue.Client, config PieceListConfig) (*PieceList, error) {
	nodeID, err := storj.NodeIDFromString(config.Node)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	config.nodeID = nodeID
	return &PieceList{
		config: config,
		client: client,
	}, nil
}

// Start is called at the beginning of each segment loop.
func (o *PieceList) Start(ctx context.Context, startTime time.Time) error {
	return nil
}

// Fork creates a new partial for processing a range.
func (o *PieceList) Fork(ctx context.Context) (rangedloop.Partial, error) {
	return &pieceListFork{
		config: o.config,
		client: o.client,
	}, nil
}

// Join merges partial results. No-op since we don't track stats.
func (o *PieceList) Join(ctx context.Context, partial rangedloop.Partial) error {
	return nil
}

// Finish is called after all segments are processed.
func (o *PieceList) Finish(ctx context.Context) error {
	return nil
}

// pieceListFork implements rangedloop.Partial.
type pieceListFork struct {
	config PieceListConfig
	client *taskqueue.Client
}

// streamID is the Redis stream name used for node audit jobs.
const streamID = "nodeaudit"

// Process handles a batch of segments.
func (f *pieceListFork) Process(ctx context.Context, segments []rangedloop.Segment) error {
	var jobs []any

	for _, segment := range segments {
		if segment.Inline() {
			continue
		}

		pieceNo, found := findPieceByNodeID(segment.Pieces, f.config.nodeID)
		if !found {
			continue
		}
		job := Job{
			StreamID:    segment.StreamID,
			Position:    segment.Position.Encode(),
			PieceNo:     pieceNo,
			RootPieceID: segment.RootPieceID,
		}
		jobs = append(jobs, job)
		if len(jobs) >= 10 {
			err := f.client.PushBatch(ctx, streamID, jobs)
			if err != nil {
				return err
			}
			jobs = jobs[:0]
		}
	}

	if len(jobs) == 0 {
		return nil
	}

	return f.client.PushBatch(ctx, streamID, jobs)
}

func findPieceByNodeID(pieces metabase.Pieces, nodeID storj.NodeID) (uint16, bool) {
	for _, piece := range pieces {
		if piece.StorageNode == nodeID {
			return piece.Number, true
		}
	}
	return 0, false
}
