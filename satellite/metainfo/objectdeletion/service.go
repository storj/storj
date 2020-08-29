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
)

const (
	lastSegmentIndex  = -1
	firstSegmentIndex = 0
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
	GetItems(ctx context.Context, paths [][]byte) ([]*pb.Pointer, error)
	UnsynchronizedGetDel(ctx context.Context, paths [][]byte) (deletedPaths [][]byte, _ []*pb.Pointer, _ error)
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
func (service *Service) Delete(ctx context.Context, requests ...*ObjectIdentifier) (reports []Report, err error) {
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
		pointers, paths, err := service.DeletePointers(ctx, requests[:batchSize])
		if err != nil {
			return reports, Error.Wrap(err)
		}

		report := GenerateReport(ctx, service.log, requests[:batchSize], paths, pointers)
		reports = append(reports, report)
		requests = requests[batchSize:]
	}

	return reports, nil
}

// DeletePointers returns a list of pointers and their paths that are deleted.
// If a object is not found, we will consider it as a successful delete.
func (service *Service) DeletePointers(ctx context.Context, requests []*ObjectIdentifier) (_ []*pb.Pointer, _ [][]byte, err error) {
	defer mon.Task()(&ctx, len(requests))(&err)

	// get first and last segment to determine the object state
	lastAndFirstSegmentsPath := [][]byte{}
	for _, req := range requests {
		lastSegmentPath, err := req.SegmentPath(lastSegmentIndex)
		if err != nil {
			return nil, nil, Error.Wrap(err)
		}
		firstSegmentPath, err := req.SegmentPath(firstSegmentIndex)
		if err != nil {
			return nil, nil, Error.Wrap(err)
		}
		lastAndFirstSegmentsPath = append(lastAndFirstSegmentsPath,
			lastSegmentPath,
			firstSegmentPath,
		)
	}

	// Get pointers from the database
	pointers, err := service.pointers.GetItems(ctx, lastAndFirstSegmentsPath)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	states, err := CreateObjectStates(ctx, requests, pointers, lastAndFirstSegmentsPath)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	pathsToDel, err := service.generateSegmentPathsForCompleteObjects(ctx, states)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	zombiePaths, err := service.collectSegmentPathsForZombieObjects(ctx, states)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}
	pathsToDel = append(pathsToDel, zombiePaths...)

	// Delete pointers and fetch the piece ids.
	//
	// The deletion may fail in the database for an arbitrary reason.
	// In that case we return an error and the pointers are left intact.
	//
	// The deletion may succeed in the database, but the connection may drop
	// while the database is sending a response -- in that case we won't send
	// the piecedeletion requests and and let garbage collection clean up those
	// pieces.
	paths, pointers, err := service.pointers.UnsynchronizedGetDel(ctx, pathsToDel)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	// if object is missing, we can consider it as a successful delete
	objectMissingPaths := service.extractSegmentPathsForMissingObjects(ctx, states)
	for _, p := range objectMissingPaths {
		paths = append(paths, p)
		pointers = append(pointers, nil)
	}

	return pointers, paths, nil
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

// generateSegmentPathsForCompleteObjects collects segment paths for objects that has last segment found in pointerDB.
func (service *Service) generateSegmentPathsForCompleteObjects(ctx context.Context, states map[string]*ObjectState) (_ [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)

	segmentPaths := [][]byte{}

	for _, state := range states {
		switch state.Status() {
		case ObjectMissing:
			// nothing to do here
		case ObjectMultiSegment:
			// just delete the starting things, we already have the necessary information
			lastSegmentPath, err := state.SegmentPath(lastSegmentIndex)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			firstSegmentPath, err := state.SegmentPath(firstSegmentIndex)
			if err != nil {
				return nil, Error.Wrap(err)
			}

			segmentPaths = append(segmentPaths, lastSegmentPath)
			segmentPaths = append(segmentPaths, firstSegmentPath)

			largestSegmentIdx, err := numberOfSegmentsFromPointer(state.LastSegment)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			// gather all segment paths that're not first or last segments
			for index := largestSegmentIdx - 1; index > firstSegmentIndex; index-- {
				path, err := state.SegmentPath(index)
				if err != nil {
					return nil, Error.Wrap(err)
				}
				segmentPaths = append(segmentPaths, path)
			}
		case ObjectSingleSegment:
			// just add to segment path, we already have the necessary information
			lastSegmentPath, err := state.SegmentPath(lastSegmentIndex)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			segmentPaths = append(segmentPaths, lastSegmentPath)
		case ObjectActiveOrZombie:
			// we will handle it in a separate method
		}
	}

	return segmentPaths, nil
}

// collectSegmentPathsForZombieObjects collects segment paths for objects that has no last segment found in pointerDB.
func (service *Service) collectSegmentPathsForZombieObjects(ctx context.Context, states map[string]*ObjectState) (_ [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)

	zombies := map[string]ObjectIdentifier{}
	largestLoaded := map[string]int64{}

	segmentsToDel := [][]byte{}

	for _, state := range states {
		if state.Status() == ObjectActiveOrZombie {
			firstSegmentPath, err := state.SegmentPath(firstSegmentIndex)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			segmentsToDel = append(segmentsToDel, firstSegmentPath)

			zombies[state.Key()] = state.ObjectIdentifier
			largestLoaded[state.Key()] = int64(firstSegmentIndex)
		}
	}

	largestKnownSegment := int64(firstSegmentIndex)
	for len(zombies) > 0 {
		// Don't make requests for segments where we found the final segment.
		for key, largest := range largestLoaded {
			if largest != largestKnownSegment {
				delete(largestLoaded, key)
				delete(zombies, key)
			}
		}
		// found all segments
		if len(zombies) == 0 {
			break
		}

		// Request the next batch of segments.
		startFrom := largestKnownSegment + 1
		largestKnownSegment += int64(service.config.ZombieSegmentsPerRequest)

		var zombieSegmentPaths [][]byte
		for _, id := range zombies {
			for i := startFrom; i <= largestKnownSegment; i++ {
				path, err := id.SegmentPath(i)
				if err != nil {
					return nil, Error.Wrap(err)
				}
				zombieSegmentPaths = append(zombieSegmentPaths, path)
			}
		}

		// We are relying on the database to return the pointers in the same
		// order as the paths we requested.
		pointers, err := service.pointers.GetItems(ctx, zombieSegmentPaths)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		for i, p := range zombieSegmentPaths {
			if pointers[i] == nil {
				continue
			}
			id, segmentIdx, err := ParseSegmentPath(p)
			if err != nil {
				return nil, Error.Wrap(err)
			}

			segmentsToDel = append(segmentsToDel, p)
			largestLoaded[id.Key()] = max(largestLoaded[id.Key()], segmentIdx)
		}
	}

	return segmentsToDel, nil
}

func (service *Service) extractSegmentPathsForMissingObjects(ctx context.Context, states map[string]*ObjectState) [][]byte {
	paths := make([][]byte, 0, len(states))
	for _, state := range states {
		if state.Status() == ObjectMissing {
			lastSegmentPath, err := state.ObjectIdentifier.SegmentPath(lastSegmentIndex)
			if err != nil {
				// it shouldn't happen
				service.log.Debug("failed to get segment path for missing object",
					zap.Stringer("Project ID", state.ObjectIdentifier.ProjectID),
					zap.String("Bucket", string(state.ObjectIdentifier.Bucket)),
					zap.String("Encrypted Path", string(state.ObjectIdentifier.EncryptedPath)),
				)
				continue
			}
			paths = append(paths, lastSegmentPath)
		}
	}

	return paths
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
