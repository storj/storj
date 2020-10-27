// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"strconv"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/storage"
)

const (
	lastSegment = int(-1)
	rateLimit   = 0
)

// object represents object with segments.
type object struct {
	segments                 bitArray
	expectedNumberOfSegments int
	hasLastSegment           bool
	// if skip is true then segments from this object shouldn't be treated as zombie segments
	// and printed out, e.g. when one of segments is out of specified date rage
	skip bool
}

// bucketsObjects keeps a list of objects associated with their path per bucket
// name.
type bucketsObjects map[string]map[metabase.ObjectKey]*object

func newObserver(db metainfo.PointerDB, w *csv.Writer, from, to *time.Time) (*observer, error) {
	headers := []string{
		"ProjectID",
		"SegmentIndex",
		"Bucket",
		"EncodedEncryptedPath",
		"CreationDate",
		"Size",
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
		zombieBuffer: make([]int, 0),

		objects: make(bucketsObjects),
	}, nil
}

// observer metainfo.Loop observer for zombie reaper.
type observer struct {
	db     metainfo.PointerDB
	writer *csv.Writer
	from   *time.Time
	to     *time.Time

	lastProjectID uuid.UUID
	zombieBuffer  []int

	objects            bucketsObjects
	inlineSegments     int
	lastInlineSegments int
	remoteSegments     int
	zombieSegments     int
}

// RemoteSegment processes a segment to collect data needed to detect zombie segment.
func (obsvr *observer) RemoteSegment(ctx context.Context, segment *metainfo.Segment) (err error) {
	return obsvr.processSegment(ctx, segment)
}

// InlineSegment processes a segment to collect data needed to detect zombie segment.
func (obsvr *observer) InlineSegment(ctx context.Context, segment *metainfo.Segment) (err error) {
	return obsvr.processSegment(ctx, segment)
}

// Object not used in this implementation.
func (obsvr *observer) Object(ctx context.Context, object *metainfo.Object) (err error) {
	return nil
}

// processSegment aggregates, in the observer internal state, the objects that
// belong the same project, tracking their segments indexes and aggregated
// information of them for calling analyzeProject method, before a new project
// list of object segments starts and its internal status is reset.
//
// It also aggregates some stats about all the segments independently of the
// object to which belong.
//
// NOTE it's expected that this method is called continually for the objects
// which belong to a same project before calling it with objects of another
// project.
func (obsvr *observer) processSegment(ctx context.Context, segment *metainfo.Segment) error {
	if !obsvr.lastProjectID.IsZero() && obsvr.lastProjectID != segment.Location.ProjectID {
		err := obsvr.analyzeProject(ctx)
		if err != nil {
			return err
		}

		// cleanup map to free memory
		obsvr.clearBucketsObjects()
	}

	obsvr.lastProjectID = segment.Location.ProjectID
	isLastSegment := segment.Location.IsLast()

	// collect number of pointers for reporting
	if segment.Inline {
		obsvr.inlineSegments++
		if isLastSegment {
			obsvr.lastInlineSegments++
		}
	} else {
		obsvr.remoteSegments++
	}

	object := findOrCreate(segment.Location.BucketName, segment.Location.ObjectKey, obsvr.objects)
	if obsvr.from != nil && segment.CreationDate.Before(*obsvr.from) {
		object.skip = true
		// release the memory consumed by the segments because it won't be used
		// for skip objects
		object.segments = nil
		return nil
	} else if obsvr.to != nil && segment.CreationDate.After(*obsvr.to) {
		object.skip = true
		// release the memory consumed by the segments because it won't be used
		// for skip objects
		object.segments = nil
		return nil
	}

	if isLastSegment {
		object.hasLastSegment = true
		if segment.MetadataNumberOfSegments > 0 {
			object.expectedNumberOfSegments = segment.MetadataNumberOfSegments
		}
	} else {
		segmentIndex := int(segment.Location.Index)
		if int64(segmentIndex) != segment.Location.Index {
			return errs.New("unsupported segment index: %d", segment.Location.Index)
		}
		ok, err := object.segments.Has(segmentIndex)
		if err != nil {
			return err
		}
		if ok {
			// TODO make location displayable
			return errs.New("fatal error this segment is duplicated: %s", segment.Location.Encode())
		}

		err = object.segments.Set(segmentIndex)
		if err != nil {
			return err
		}
	}

	return nil
}

func (obsvr *observer) detectZombieSegments(ctx context.Context) error {
	err := metainfo.IterateDatabase(ctx, rateLimit, obsvr.db, obsvr)
	if err != nil {
		return err
	}

	return obsvr.analyzeProject(ctx)
}

// analyzeProject analyzes the objects in obsv.objects field for detecting bad
// segments and writing them to objs.writer.
func (obsvr *observer) analyzeProject(ctx context.Context) error {
	for bucket, objects := range obsvr.objects {
		for key, object := range objects {
			if object.skip {
				continue
			}

			err := obsvr.findZombieSegments(object)
			if err != nil {
				return err
			}

			for _, segmentIndex := range obsvr.zombieBuffer {
				err = obsvr.printSegment(ctx, segmentIndex, bucket, key)
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

	switch {
	// this case is only for old style pointers with encrypted number of segments
	// value 0 means that we don't know how much segments object should have
	case object.expectedNumberOfSegments == 0:
		sequenceLength := firstSequenceLength(object.segments)

		for index := sequenceLength; index < object.segments.Length(); index++ {
			has, err := object.segments.Has(index)
			if err != nil {
				panic(err)
			}
			if has {
				obsvr.appendSegment(index)
			}
		}
	// using 'expectedNumberOfSegments-1' because 'segments' doesn't contain last segment
	case segmentsCount > object.expectedNumberOfSegments-1:
		sequenceLength := firstSequenceLength(object.segments)

		if sequenceLength == object.expectedNumberOfSegments-1 {
			for index := sequenceLength; index < object.segments.Length(); index++ {
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
	case segmentsCount < object.expectedNumberOfSegments-1,
		segmentsCount == object.expectedNumberOfSegments-1 && !object.segments.IsSequence():
		obsvr.appendAllObjectSegments(object)
		obsvr.appendSegment(lastSegment)
	}

	return nil
}

func (obsvr *observer) printSegment(ctx context.Context, segmentIndex int, bucket string, key metabase.ObjectKey) error {
	var segmentIndexStr string
	if segmentIndex == lastSegment {
		segmentIndexStr = "l"
	} else {
		segmentIndexStr = "s" + strconv.Itoa(segmentIndex)
	}

	segmentKey := metabase.SegmentLocation{
		ProjectID:  obsvr.lastProjectID,
		BucketName: bucket,
		Index:      int64(segmentIndex),
		ObjectKey:  key,
	}.Encode()
	creationDate, size, err := pointerCreationDateAndSize(ctx, obsvr.db, segmentKey)
	if err != nil {
		return err
	}
	encodedPath := base64.StdEncoding.EncodeToString([]byte(key))
	err = obsvr.writer.Write([]string{
		obsvr.lastProjectID.String(),
		segmentIndexStr,
		bucket,
		encodedPath,
		creationDate,
		strconv.FormatInt(size, 10),
	})
	if err != nil {
		return err
	}
	obsvr.zombieSegments++
	return nil
}

func pointerCreationDateAndSize(ctx context.Context, db metainfo.PointerDB, key metabase.SegmentKey,
) (creationDate string, size int64, _ error) {
	pointerBytes, err := db.Get(ctx, storage.Key(key))
	if err != nil {
		return "", 0, err
	}

	pointer := &pb.Pointer{}
	err = pb.Unmarshal(pointerBytes, pointer)
	if err != nil {
		return "", 0, err
	}

	return pointer.CreationDate.Format(time.RFC3339Nano), pointer.SegmentSize, nil
}

func (obsvr *observer) resetZombieBuffer() {
	obsvr.zombieBuffer = obsvr.zombieBuffer[:0]
}

func (obsvr *observer) appendSegment(segmentIndex int) {
	obsvr.zombieBuffer = append(obsvr.zombieBuffer, segmentIndex)
}

func (obsvr *observer) appendAllObjectSegments(object *object) {
	for index := 0; index < object.segments.Length(); index++ {
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

func findOrCreate(bucketName string, key metabase.ObjectKey, buckets bucketsObjects) *object {
	objects, ok := buckets[bucketName]
	if !ok {
		objects = make(map[metabase.ObjectKey]*object)
		buckets[bucketName] = objects
	}

	obj, ok := objects[key]
	if !ok {
		obj = &object{segments: bitArray{}}
		objects[key] = obj
	}

	return obj
}

func firstSequenceLength(segments bitArray) int {
	for index := 0; index < segments.Length(); index++ {
		has, err := segments.Has(index)
		if err != nil {
			panic(err)
		}
		if !has {
			return index
		}
	}
	return segments.Length()
}
