// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"os"
	"strconv"

	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
)

const maxNumOfSegments = byte(64)

var (
	detectCmd = &cobra.Command{
		Use:   "detect",
		Short: "Detects zombie segments in DB",
		Args:  cobra.OnlyValidArgs,
		RunE:  cmdDetect,
	}

	detectCfg struct {
		DatabaseURL string `help:"the database connection string to use" default:"postgres://"`
		From        string `help:"begin of date range for detecting zombie segments" default:""`
		To          string `help:"end of date range for detecting zombie segments" default:""`
		File        string `help:"location of file with report" default:"zombie-segments.csv"`
	}
)

// Cluster key for objects map.
type Cluster struct {
	projectID string
	bucket    string
}

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

// ObjectsMap map that keeps objects representation.
type ObjectsMap map[Cluster]map[storj.Path]*Object

// Observer metainfo.Loop observer for zombie reaper.
type Observer struct {
	db      metainfo.PointerDB
	objects ObjectsMap
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
	cluster := Cluster{
		projectID: path.ProjectIDString,
		bucket:    path.BucketName,
	}

	if observer.lastProjectID != "" && observer.lastProjectID != cluster.projectID {
		err := analyzeProject(ctx, observer.db, observer.objects, observer.writer)
		if err != nil {
			return err
		}

		// cleanup map to free memory
		observer.objects = make(ObjectsMap)
	}

	isLastSegment := path.Segment == "l"
	object := findOrCreate(cluster, path.EncryptedObjectPath, observer.objects)
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
	observer.lastProjectID = cluster.projectID
	return nil
}

func init() {
	rootCmd.AddCommand(detectCmd)

	defaults := cfgstruct.DefaultsFlag(rootCmd)
	process.Bind(detectCmd, &detectCfg, defaults)
}

func cmdDetect(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	log := zap.L()
	db, err := metainfo.NewStore(log.Named("pointerdb"), detectCfg.DatabaseURL)
	if err != nil {
		return errs.New("error connecting database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	file, err := os.Create(detectCfg.File)
	if err != nil {
		return errs.New("error creating result file: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, file.Close())
	}()

	writer := csv.NewWriter(file)
	defer func() {
		writer.Flush()
		err = errs.Combine(err, writer.Error())
	}()

	headers := []string{
		"ProjectID",
		"SegmentIndex",
		"Bucket",
		"EncodedEncryptedPath",
		"CreationDate",
	}
	err = writer.Write(headers)
	if err != nil {
		return err
	}

	observer := &Observer{
		objects: make(ObjectsMap),
		db:      db,
		writer:  writer,
	}
	err = metainfo.IterateDatabase(ctx, db, observer)
	if err != nil {
		return err
	}

	log.Info("number of inline segments", zap.Int("segments", observer.inlineSegments))
	log.Info("number of last inline segments", zap.Int("segments", observer.lastInlineSegments))
	log.Info("number of remote segments", zap.Int("segments", observer.remoteSegments))
	return nil
}

func analyzeProject(ctx context.Context, db metainfo.PointerDB, objectsMap ObjectsMap, csvWriter *csv.Writer) error {
	// TODO this part will be implemented in next PR
	return nil
}

func findOrCreate(cluster Cluster, path string, objects ObjectsMap) *Object {
	objectsMap, ok := objects[cluster]
	if !ok {
		objectsMap = make(map[storj.Path]*Object)
		objects[cluster] = objectsMap
	}

	object, ok := objectsMap[path]
	if !ok {
		object = &Object{}
		objectsMap[path] = object
	}

	return object
}
