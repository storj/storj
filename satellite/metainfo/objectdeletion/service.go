// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package objectdeletion

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metainfo/metabase"
)

var (
	mon = monkit.Package()
	// Error is a general object deletion error.
	Error = errs.Class("object deletion")
)

// Config defines configuration options for Service.
type Config struct {
	MaxObjectsPerRequest     int `help:"maximum number of requests per batch" default:"100"`
	ZombieSegmentsPerRequest int `help:"number of segments per request when looking for zombie segments" default:"3"`
	MaxConcurrentRequests    int `help:"maximum number of concurrent requests" default:"10000"`
}

// Verify verifies configuration sanity.
func (config *Config) Verify() errs.Group {
	var errlist errs.Group

	if config.MaxObjectsPerRequest <= 0 {
		errlist.Add(Error.New("max requests per batch %d must be greater than 0", config.MaxObjectsPerRequest))
	}

	if config.ZombieSegmentsPerRequest <= 0 {
		errlist.Add(Error.New("zombie segments per request %d must be greater than 0", config.ZombieSegmentsPerRequest))
	}

	if config.MaxConcurrentRequests <= 0 {
		errlist.Add(Error.New("max concurrent requests %d must be greater than 0", config.MaxConcurrentRequests))
	}

	return errlist
}

// PointerDB stores pointers.
type PointerDB interface {
	GetItems(ctx context.Context, keys []metabase.SegmentKey) ([]*pb.Pointer, error)
	UnsynchronizedGetDel(ctx context.Context, keys []metabase.SegmentKey) (deletedKeys []metabase.SegmentKey, _ []*pb.Pointer, _ error)
}

// Service implements the object deletion service
//
// architecture: Service
type Service struct {
	log    *zap.Logger
	config Config

	concurrentRequests *semaphore.Weighted

	pointers PointerDB
}

// NewService returns new instance of Service.
func NewService(log *zap.Logger, pointerDB PointerDB, config Config) (*Service, error) {
	if errs := config.Verify(); len(errs) > 0 {
		return nil, errs.Err()
	}

	return &Service{
		log:                log,
		config:             config,
		concurrentRequests: semaphore.NewWeighted(int64(config.MaxConcurrentRequests)),
		pointers:           pointerDB,
	}, nil
}

// Delete ensures that all pointers belongs to an object no longer exists.
func (service *Service) Delete(ctx context.Context, requests ...*metabase.ObjectLocation) (reports []Report, err error) {
	defer mon.Task()(&ctx, len(requests))(&err)

	// When number of requests are more than the maximum limit, we let it overflow,
	// so we don't have to acquire the semaphore on each batch.
	requestsCount := len(requests)
	if requestsCount > service.config.MaxConcurrentRequests {
		requestsCount = service.config.MaxConcurrentRequests
	}
	if err := service.concurrentRequests.Acquire(ctx, int64(requestsCount)); err != nil {
		return reports, Error.Wrap(err)
	}
	defer service.concurrentRequests.Release(int64(requestsCount))

	for len(requests) > 0 {
		batchSize := len(requests)
		if batchSize > service.config.MaxObjectsPerRequest {
			batchSize = service.config.MaxObjectsPerRequest
		}
		pointers, keys, err := service.DeletePointers(ctx, requests[:batchSize])
		if err != nil {
			return reports, Error.Wrap(err)
		}

		report := GenerateReport(ctx, service.log, requests[:batchSize], keys, pointers)
		reports = append(reports, report)
		requests = requests[batchSize:]
	}

	return reports, nil
}

// DeletePointers returns a list of pointers and their keys that are deleted.
// If a object is not found, we will consider it as a successful delete.
func (service *Service) DeletePointers(ctx context.Context, requests []*metabase.ObjectLocation) (_ []*pb.Pointer, _ []metabase.SegmentKey, err error) {
	defer mon.Task()(&ctx, len(requests))(&err)

	// get first and last segment to determine the object state
	lastAndFirstSegmentsKey := []metabase.SegmentKey{}
	for _, req := range requests {
		lastAndFirstSegmentsKey = append(lastAndFirstSegmentsKey,
			req.LastSegment().Encode(),
			req.FirstSegment().Encode(),
		)
	}

	// Get pointers from the database
	pointers, err := service.pointers.GetItems(ctx, lastAndFirstSegmentsKey)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	states, err := CreateObjectStates(ctx, requests, pointers, lastAndFirstSegmentsKey)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	keysToDel, err := service.generateSegmentKeysForCompleteObjects(ctx, states)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	zombieKeys, err := service.collectSegmentKeysForZombieObjects(ctx, states)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}
	keysToDel = append(keysToDel, zombieKeys...)

	// Delete pointers and fetch the piece ids.
	//
	// The deletion may fail in the database for an arbitrary reason.
	// In that case we return an error and the pointers are left intact.
	//
	// The deletion may succeed in the database, but the connection may drop
	// while the database is sending a response -- in that case we won't send
	// the piecedeletion requests and and let garbage collection clean up those
	// pieces.
	keys, pointers, err := service.pointers.UnsynchronizedGetDel(ctx, keysToDel)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	// if object is missing, we can consider it as a successful delete
	objectMissingKeys := service.extractSegmentKeysForMissingObjects(ctx, states)
	for _, p := range objectMissingKeys {
		keys = append(keys, p)
		pointers = append(pointers, nil)
	}

	return pointers, keys, nil
}

// GroupPiecesByNodeID returns a map that contains pieces with node id as the key.
func GroupPiecesByNodeID(pointers []*pb.Pointer) map[storj.NodeID][]storj.PieceID {
	// build piece deletion requests
	piecesToDelete := map[storj.NodeID][]storj.PieceID{}
	for _, p := range pointers {
		if p == nil {
			continue
		}
		if p.Type != pb.Pointer_REMOTE {
			continue
		}

		rootPieceID := p.GetRemote().RootPieceId
		for _, piece := range p.GetRemote().GetRemotePieces() {
			pieceID := rootPieceID.Derive(piece.NodeId, piece.PieceNum)
			piecesToDelete[piece.NodeId] = append(piecesToDelete[piece.NodeId], pieceID)
		}
	}

	return piecesToDelete
}

// generateSegmentKeysForCompleteObjects collects segment keys for objects that has last segment found in pointerDB.
func (service *Service) generateSegmentKeysForCompleteObjects(ctx context.Context, states map[metabase.ObjectLocation]*ObjectState) (_ []metabase.SegmentKey, err error) {
	defer mon.Task()(&ctx)(&err)

	segmentKeys := []metabase.SegmentKey{}

	for _, state := range states {
		switch state.Status() {
		case ObjectMissing:
			// nothing to do here
		case ObjectMultiSegment:
			segmentKeys = append(segmentKeys,
				state.ObjectLocation.LastSegment().Encode(),
				state.ObjectLocation.FirstSegment().Encode())

			largestSegmentIdx, err := numberOfSegmentsFromPointer(state.LastSegment)
			if err != nil {
				return nil, Error.Wrap(err)
			}

			// gather all segment keys that're not first or last segments
			for index := largestSegmentIdx - 1; index > metabase.FirstSegmentIndex; index-- {
				location, err := state.Segment(index)
				if err != nil {
					return nil, Error.Wrap(err)
				}
				segmentKeys = append(segmentKeys, location.Encode())
			}

		case ObjectSingleSegment:
			// just add to segment key, we already have the necessary information
			segmentKeys = append(segmentKeys, state.ObjectLocation.LastSegment().Encode())
		case ObjectActiveOrZombie:
			// we will handle it in a separate method
		}
	}

	return segmentKeys, nil
}

// collectSegmentKeysForZombieObjects collects segment keys for objects that has no last segment found in pointerDB.
func (service *Service) collectSegmentKeysForZombieObjects(ctx context.Context, states map[metabase.ObjectLocation]*ObjectState) (_ []metabase.SegmentKey, err error) {
	defer mon.Task()(&ctx)(&err)

	largestLoaded := map[metabase.ObjectLocation]int64{}

	segmentsToDel := []metabase.SegmentKey{}

	for _, state := range states {
		if state.Status() == ObjectActiveOrZombie {
			segmentsToDel = append(segmentsToDel, state.ObjectLocation.FirstSegment().Encode())

			largestLoaded[state.ObjectLocation] = metabase.FirstSegmentIndex
		}
	}

	largestKnownSegment := int64(metabase.FirstSegmentIndex)
	for len(largestLoaded) > 0 {
		// Don't make requests for segments where we found the final segment.
		for key, largest := range largestLoaded {
			if largest != largestKnownSegment {
				delete(largestLoaded, key)
			}
		}
		// found all segments
		if len(largestLoaded) == 0 {
			break
		}

		// Request the next batch of segments.
		startFrom := largestKnownSegment + 1
		largestKnownSegment += int64(service.config.ZombieSegmentsPerRequest)

		var zombieSegmentKeys []metabase.SegmentKey
		for location := range largestLoaded {
			for i := startFrom; i <= largestKnownSegment; i++ {
				location, err := location.Segment(i)
				if err != nil {
					return nil, Error.Wrap(err)
				}
				zombieSegmentKeys = append(zombieSegmentKeys, location.Encode())
			}
		}

		// We are relying on the database to return the pointers in the same
		// order as the keys we requested.
		pointers, err := service.pointers.GetItems(ctx, zombieSegmentKeys)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		for i, p := range zombieSegmentKeys {
			if pointers[i] == nil {
				continue
			}
			segmentLocation, err := metabase.ParseSegmentKey(p)
			if err != nil {
				return nil, Error.Wrap(err)
			}

			segmentsToDel = append(segmentsToDel, p)
			objectKey := segmentLocation.Object()
			largestLoaded[objectKey] = max(largestLoaded[objectKey], segmentLocation.Index)
		}
	}

	return segmentsToDel, nil
}

func (service *Service) extractSegmentKeysForMissingObjects(ctx context.Context, states map[metabase.ObjectLocation]*ObjectState) []metabase.SegmentKey {
	keys := make([]metabase.SegmentKey, 0, len(states))
	for _, state := range states {
		if state.Status() == ObjectMissing {
			keys = append(keys, state.ObjectLocation.LastSegment().Encode())
		}
	}
	return keys
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func numberOfSegmentsFromPointer(pointer *pb.Pointer) (int64, error) {
	meta := &pb.StreamMeta{}
	err := pb.Unmarshal(pointer.Metadata, meta)
	if err != nil {
		return 0, err
	}

	return meta.NumberOfSegments, nil
}
