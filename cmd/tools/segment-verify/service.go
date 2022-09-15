// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

var mon = monkit.Package()

// Error is global error class.
var Error = errs.Class("segment-verify")

// VerifyPieces defines how many pieces we check per segment.
const VerifyPieces = 3

// ConcurrentRequests defines how many concurrent requests we do to the storagenodes.
const ConcurrentRequests = 10000

// Metabase defines implementation dependencies we need from metabase.
type Metabase interface {
	ConvertNodesToAliases(ctx context.Context, nodeID []storj.NodeID) ([]metabase.NodeAlias, error)
	ConvertAliasesToNodes(ctx context.Context, aliases []metabase.NodeAlias) ([]storj.NodeID, error)

	GetSegmentByPosition(ctx context.Context, opts metabase.GetSegmentByPosition) (segment metabase.Segment, err error)
	ListVerifySegments(ctx context.Context, opts metabase.ListVerifySegments) (result metabase.ListVerifySegmentsResult, err error)
}

// SegmentWriter allows writing segments to some output.
type SegmentWriter interface {
	Write(ctx context.Context, segments []*Segment) error
}

// Service implements segment verification logic.
type Service struct {
	log      *zap.Logger
	metabase Metabase

	notFound SegmentWriter
	retry    SegmentWriter

	PriorityNodes NodeAliasSet
	OfflineNodes  NodeAliasSet
}

// NewService returns a new service for verifying segments.
func NewService(log *zap.Logger) *Service {
	return &Service{
		log: log,

		PriorityNodes: NodeAliasSet{},
		OfflineNodes:  NodeAliasSet{},
	}
}

// Process processes segments between low and high uuid.UUID with the specified batchSize.
func (service *Service) Process(ctx context.Context, low, high uuid.UUID, batchSize int) (err error) {
	defer mon.Task()(&ctx)(&err)

	cursorStreamID := low
	if !low.IsZero() {
		cursorStreamID = uuidBefore(low)
	}
	cursorPosition := metabase.SegmentPosition{Part: 0xFFFFFFFF, Index: 0xFFFFFFFF}

	for {
		result, err := service.metabase.ListVerifySegments(ctx, metabase.ListVerifySegments{
			CursorStreamID: cursorStreamID,
			CursorPosition: cursorPosition,
			Limit:          batchSize,

			// TODO: add AS OF SYSTEM time.
		})
		if err != nil {
			return Error.Wrap(err)
		}
		verifySegments := result.Segments
		result.Segments = nil

		// drop any segment that's equal or beyond "high".
		for len(verifySegments) > 0 && !verifySegments[len(verifySegments)-1].StreamID.Less(high) {
			verifySegments = verifySegments[:len(verifySegments)-1]
		}

		// All done?
		if len(verifySegments) == 0 {
			return nil
		}

		// Convert to struct that contains the status.
		segmentsData := make([]Segment, len(verifySegments))
		segments := make([]*Segment, len(verifySegments))
		for i := range segments {
			segmentsData[i].VerifySegment = verifySegments[i]
			segments[i] = &segmentsData[i]
		}

		// Process the data.
		err = service.ProcessSegments(ctx, segments)
		if err != nil {
			return Error.Wrap(err)
		}
	}
}

// ProcessSegments processes a collection of segments.
func (service *Service) ProcessSegments(ctx context.Context, segments []*Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	service.log.Info("processing segments",
		zap.Int("count", len(segments)),
		zap.Stringer("first", segments[0].StreamID),
		zap.Stringer("last", segments[len(segments)-1].StreamID),
	)

	// Verify all the segments against storage nodes.
	err = service.Verify(ctx, segments)
	if err != nil {
		return Error.Wrap(err)
	}

	notFound := []*Segment{}
	retry := []*Segment{}

	// Find out which of the segments we did not find
	// or there was some other failure.
	for _, segment := range segments {
		if segment.Status.NotFound > 0 {
			notFound = append(notFound, segment)
		} else if segment.Status.Retry > 0 {
			// TODO: should we do a smarter check here?
			// e.g. if at least half did find, then consider it ok?
			retry = append(retry, segment)
		}
	}

	// Some segments might have been deleted during the
	// processing, so cross-reference and remove any deleted
	// segments from the list.
	notFound, err = service.RemoveDeleted(ctx, notFound)
	if err != nil {
		return Error.Wrap(err)
	}
	retry, err = service.RemoveDeleted(ctx, retry)
	if err != nil {
		return Error.Wrap(err)
	}

	// Output the problematic segments:
	errNotFound := service.notFound.Write(ctx, notFound)
	errRetry := service.retry.Write(ctx, retry)

	return errs.Combine(errNotFound, errRetry)
}

// RemoveDeleted modifies the slice and returns only the segments that
// still exist in the database.
func (service *Service) RemoveDeleted(ctx context.Context, segments []*Segment) (_ []*Segment, err error) {
	defer mon.Task()(&ctx)(&err)

	valid := segments[:0]
	for _, seg := range segments {
		_, err := service.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: seg.StreamID,
			Position: seg.Position,
		})
		if metabase.ErrSegmentNotFound.Has(err) {
			continue
		}
		if err != nil {
			service.log.Error("get segment by id failed", zap.Stringer("stream-id", seg.StreamID), zap.String("position", fmt.Sprint(seg.Position)))
			if ctx.Err() != nil {
				return valid, ctx.Err()
			}
		}
		valid = append(valid, seg)
	}
	return valid, nil
}

// Segment contains minimal information necessary for verifying a single Segment.
type Segment struct {
	metabase.VerifySegment
	Status Status
}

// Status contains the statistics about the segment.
type Status struct {
	Retry    int32
	Found    int32
	NotFound int32
}

// MarkFound moves a retry token from retry to found.
func (status *Status) MarkFound() {
	atomic.AddInt32(&status.Retry, -1)
	atomic.AddInt32(&status.Found, 1)
}

// MarkNotFound moves a retry token from retry to not found.
func (status *Status) MarkNotFound() {
	atomic.AddInt32(&status.Retry, -1)
	atomic.AddInt32(&status.NotFound, 1)
}

// Batch is a list of segments to be verified on a single node.
type Batch struct {
	Alias metabase.NodeAlias
	Items []*Segment
}

// Len returns the length of the batch.
func (b *Batch) Len() int { return len(b.Items) }

// uuidBefore returns an uuid.UUID that's immediately before v.
// It might not be a valid uuid after this operation.
func uuidBefore(v uuid.UUID) uuid.UUID {
	for i := len(v) - 1; i >= 0; i-- {
		v[i]--
		if v[i] != 0xFF { // we didn't wrap around
			break
		}
	}
	return v
}
