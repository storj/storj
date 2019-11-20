package main

import (
	"context"
	"encoding/csv"
	"strconv"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
)

// Object represents object with segments.
type Object struct {
	// TODO verify if we have more than 64 segments for object in network
	segments uint64

	expectedNumberOfSegments byte

	hasLastSegment bool
	// if skip is true then segments from this object shouldn't be treated as zombie segments
	// and printed out, e.g. when one of segments is out of specified date rage
	skip bool
}

// bucketsObjects keeps a list of objects associated with their path per bucket
// name.
type bucketsObjects map[string]map[storj.Path]*Object

// Observer metainfo.Loop observer for zombie reaper.
type Observer struct {
	db      metainfo.PointerDB
	objects bucketsObjects
	writer  *csv.Writer

	lastProjectID string

	inlineSegments     int
	lastInlineSegments int
	remoteSegments     int
}

// RemoteSegment processes a segment to collect data needed to detect zombie segment.
func (observer *Observer) RemoteSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return observer.processSegment(ctx, path, pointer)
}

// InlineSegment processes a segment to collect data needed to detect zombie segment.
func (observer *Observer) InlineSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return observer.processSegment(ctx, path, pointer)
}

// Object not used in this implementation.
func (observer *Observer) Object(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return nil
}

func (observer *Observer) processSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) error {
	if observer.lastProjectID != "" && observer.lastProjectID != path.ProjectIDString {
		err := analyzeProject(ctx, observer.db, observer.lastProjectID, observer.objects, observer.writer)
		if err != nil {
			return err
		}

		// cleanup map to free memory
		observer.objects = make(bucketsObjects)
	}

	observer.lastProjectID = path.ProjectIDString

	isLastSegment := path.Segment == "l"
	object := findOrCreate(path.BucketName, path.EncryptedObjectPath, observer.objects)
	if isLastSegment {
		object.hasLastSegment = true

		streamMeta := pb.StreamMeta{}
		err := proto.Unmarshal(pointer.Metadata, &streamMeta)
		if err != nil {
			return errs.New("unexpected error unmarshalling pointer metadata %s", err)
		}

		if streamMeta.NumberOfSegments > 0 {
			if streamMeta.NumberOfSegments > int64(maxNumOfSegments) {
				object.skip = true
				zap.S().Warn("unsupported number of segments", zap.Int64("index", streamMeta.NumberOfSegments))
			}
			object.expectedNumberOfSegments = byte(streamMeta.NumberOfSegments)
		}
	} else {
		segmentIndex, err := strconv.Atoi(path.Segment[1:])
		if err != nil {
			return err
		}
		if segmentIndex >= int(maxNumOfSegments) {
			object.skip = true
			zap.S().Warn("unsupported segment index", zap.Int("index", segmentIndex))
		}

		if object.segments&(1<<uint64(segmentIndex)) != 0 {
			// TODO make path displayable
			return errs.New("fatal error this segment is duplicated: %s", path.Raw)
		}

		object.segments |= 1 << uint64(segmentIndex)
	}

	// collect number of pointers for report
	if pointer.Type == pb.Pointer_INLINE {
		observer.inlineSegments++
		if isLastSegment {
			observer.lastInlineSegments++
		}
	} else {
		observer.remoteSegments++
	}

	return nil
}

func findOrCreate(bucketName string, path string, buckets bucketsObjects) *Object {
	objects, ok := buckets[bucketName]
	if !ok {
		objects = make(map[storj.Path]*Object)
		buckets[bucketName] = objects
	}

	object, ok := objects[path]
	if !ok {
		object = &Object{}
		objects[path] = object
	}

	return object
}
