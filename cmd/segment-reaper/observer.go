// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"strconv"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metainfo"
)

const (
	maxNumOfSegments = 64
	lastSegment      = int(-1)
)

// object represents object with segments.
type object struct {
	// TODO verify if we have more than 65 segments for object in network.
	// 65 because the observer tracks in the bitmask all the segments execept the
	//  last one (the 'l' segment)
	segments bitmask

	expectedNumberOfSegments byte

	hasLastSegment bool
	// if skip is true then segments from this object shouldn't be treated as zombie segments
	// and printed out, e.g. when one of segments is out of specified date rage
	skip bool
}

// bucketsObjects keeps a list of objects associated with their path per bucket
// name.
type bucketsObjects map[string]map[storj.Path]*object

func newObserver(db metainfo.PointerDB, w *csv.Writer, from, to *time.Time) (*observer, error) {
	headers := []string{
		"ProjectID",
		"SegmentIndex",
		"Bucket",
		"EncodedEncryptedPath",
		"CreationDate",
	}
	err := w.Write(headers)
	if err != nil {
		return nil, err
	}

	return &observer{
		db:           db,
		writer:       w,
		from:         from,
		to:           to,
		zombieBuffer: make([]int, 0, maxNumOfSegments),

		objects: make(bucketsObjects),
	}, nil
}

// observer metainfo.Loop observer for zombie reaper.
type observer struct {
	db     metainfo.PointerDB
	writer *csv.Writer
	from   *time.Time
	to     *time.Time

	lastProjectID string
	zombieBuffer  []int

	objects            bucketsObjects
	inlineSegments     int
	lastInlineSegments int
	remoteSegments     int
	zombieSegments     int
}

// RemoteSegment processes a segment to collect data needed to detect zombie segment.
func (obsvr *observer) RemoteSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return obsvr.processSegment(ctx, path, pointer)
}

// InlineSegment processes a segment to collect data needed to detect zombie segment.
func (obsvr *observer) InlineSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return obsvr.processSegment(ctx, path, pointer)
}

// Object not used in this implementation.
func (obsvr *observer) Object(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return nil
}

// processSegment aggregates, in the observer internal state, the objects that
// belong the same project, tracking their segments indexes and aggregated
// information of them for calling analyzeProject method, before a new project
// list of object segments list starts and the internal status is reset.
//
// It also aggregates some stats about all the segments independently of the
// object to which belong.
//
// NOTE it's expected that this method is called continually for the objects
// which belong to a same project before calling it with objects of another
// project.
func (obsvr *observer) processSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) error {
	if obsvr.lastProjectID != "" && obsvr.lastProjectID != path.ProjectIDString {
		err := obsvr.analyzeProject(ctx)
		if err != nil {
			return err
		}

		// cleanup map to free memory
		obsvr.clearBucketsObjects()
	}

	obsvr.lastProjectID = path.ProjectIDString

	isLastSegment := path.Segment == "l"
	object := findOrCreate(path.BucketName, path.EncryptedObjectPath, obsvr.objects)
	if isLastSegment {
		object.hasLastSegment = true

		streamMeta := pb.StreamMeta{}
		err := proto.Unmarshal(pointer.Metadata, &streamMeta)
		if err != nil {
			return errs.New("unexpected error unmarshalling pointer metadata %s", err)
		}

		if streamMeta.NumberOfSegments > 0 {
			// We can support the size of the bitmask + 1 because the last segment
			// ins't tracked in it.
			if streamMeta.NumberOfSegments > (int64(maxNumOfSegments) + 1) {
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
		} else {
			ok, err := object.segments.Has(segmentIndex)
			if err != nil {
				return err
			}
			if ok {
				// TODO make path displayable
				return errs.New("fatal error this segment is duplicated: %s", path.Raw)
			}

			err = object.segments.Set(segmentIndex)
			if err != nil {
				return err
			}
		}
	}

	if obsvr.from != nil && pointer.CreationDate.Before(*obsvr.from) {
		object.skip = true
	} else if obsvr.to != nil && pointer.CreationDate.After(*obsvr.to) {
		object.skip = true
	}

	// collect number of pointers for report
	if pointer.Type == pb.Pointer_INLINE {
		obsvr.inlineSegments++
		if isLastSegment {
			obsvr.lastInlineSegments++
		}
	} else {
		obsvr.remoteSegments++
	}

	return nil
}

func (obsvr *observer) detectZombieSegments(ctx context.Context) error {
	err := metainfo.IterateDatabase(ctx, obsvr.db, obsvr)
	if err != nil {
		return err
	}

	return obsvr.analyzeProject(ctx)
}

// analyzeProject analyzes the objects in obsv.objects field for detecting bad
// segments and writing them to objs.writer.
func (obsvr *observer) analyzeProject(ctx context.Context) error {
	for bucket, objects := range obsvr.objects {
		for path, object := range objects {
			if object.skip {
				continue
			}

			err := obsvr.findZombieSegments(object)
			if err != nil {
				return err
			}

			for _, segmentIndex := range obsvr.zombieBuffer {
				err = obsvr.printSegment(ctx, segmentIndex, bucket, path)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (obsvr *observer) findZombieSegments(object *object) error {
	obsvr.resetZombieBuffer()

	if !object.hasLastSegment {
		obsvr.appendAllObjectSegments(object)
		return nil
	}

	segmentsCount := object.segments.Count()

	// using 'expectedNumberOfSegments-1' because 'segments' doesn't contain last segment
	switch {
	// this case is only for old style pointers with encrypted number of segments
	// value 0 means that we don't know how much segments object should have
	case object.expectedNumberOfSegments == 0:
		sequenceLength := firstSequenceLength(object.segments)

		for index := sequenceLength; index < maxNumOfSegments; index++ {
			has, err := object.segments.Has(index)
			if err != nil {
				panic(err)
			}
			if has {
				obsvr.appendSegment(index)
			}
		}
	case segmentsCount > int(object.expectedNumberOfSegments)-1:
		sequenceLength := firstSequenceLength(object.segments)

		if sequenceLength == int(object.expectedNumberOfSegments-1) {
			for index := sequenceLength; index < maxNumOfSegments; index++ {
				has, err := object.segments.Has(index)
				if err != nil {
					panic(err)
				}
				if has {
					obsvr.appendSegment(index)
				}
			}
		} else {
			obsvr.appendAllObjectSegments(object)
			obsvr.appendSegment(lastSegment)
		}
	case segmentsCount < int(object.expectedNumberOfSegments)-1,
		segmentsCount == int(object.expectedNumberOfSegments)-1 && !object.segments.IsSequence():
		obsvr.appendAllObjectSegments(object)
		obsvr.appendSegment(lastSegment)
	}

	return nil
}

func (obsvr *observer) printSegment(ctx context.Context, segmentIndex int, bucket, path string) error {
	var segmentIndexStr string
	if segmentIndex == lastSegment {
		segmentIndexStr = "l"
	} else {
		segmentIndexStr = "s" + strconv.Itoa(segmentIndex)
	}
	creationDate, err := pointerCreationDate(ctx, obsvr.db, obsvr.lastProjectID, segmentIndexStr, bucket, path)
	if err != nil {
		return err
	}
	encodedPath := base64.StdEncoding.EncodeToString([]byte(path))
	err = obsvr.writer.Write([]string{
		obsvr.lastProjectID,
		segmentIndexStr,
		bucket,
		encodedPath,
		creationDate,
	})
	if err != nil {
		return err
	}
	obsvr.zombieSegments++
	return nil
}

func pointerCreationDate(ctx context.Context, db metainfo.PointerDB, projectID, segmentIndex, bucket, path string) (string, error) {
	key := []byte(storj.JoinPaths(projectID, segmentIndex, bucket, path))
	pointerBytes, err := db.Get(ctx, key)
	if err != nil {
		return "", err
	}

	pointer := &pb.Pointer{}
	err = proto.Unmarshal(pointerBytes, pointer)
	if err != nil {
		return "", err
	}
	return pointer.CreationDate.Format(time.RFC3339Nano), nil
}

func (obsvr *observer) resetZombieBuffer() {
	obsvr.zombieBuffer = obsvr.zombieBuffer[:0]
}

func (obsvr *observer) appendSegment(segmentIndex int) {
	obsvr.zombieBuffer = append(obsvr.zombieBuffer, segmentIndex)
}

func (obsvr *observer) appendAllObjectSegments(object *object) {
	for index := 0; index < maxNumOfSegments; index++ {
		has, err := object.segments.Has(index)
		if err != nil {
			panic(err)
		}
		if has {
			obsvr.appendSegment(index)
		}
	}
}

// clearBucketsObjects clears up the buckets objects map for reusing it.
func (obsvr *observer) clearBucketsObjects() {
	// This is an idiomatic way of not having to destroy and recreate a new map
	// each time that a empty map is required.
	// See https://github.com/golang/go/issues/20138
	for b := range obsvr.objects {
		delete(obsvr.objects, b)
	}
}

func findOrCreate(bucketName string, path string, buckets bucketsObjects) *object {
	objects, ok := buckets[bucketName]
	if !ok {
		objects = make(map[storj.Path]*object)
		buckets[bucketName] = objects
	}

	obj, ok := objects[path]
	if !ok {
		obj = &object{}
		objects[path] = obj
	}

	return obj
}

func firstSequenceLength(segments bitmask) int {
	for index := 0; index < maxNumOfSegments; index++ {
		has, err := segments.Has(index)
		if err != nil {
			panic(err)
		}
		if !has {
			return index
		}
	}
	return maxNumOfSegments
}
