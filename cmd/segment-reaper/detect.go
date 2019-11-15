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
	"storj.io/storj/storage"
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
		File        string `help:"" default:"detect_result.out"`
	}
)

// Cluster key for objects map
type Cluster struct {
	projectID string
	bucket    string
}

// Object represents object with segments
type Object struct {
	// TODO verify if we have more than 64 segments for object in network
	segments uint64

	expectedNumberOfSegments byte

	hasLastSegment bool
	// if skip is true segments from object should be removed from memory when last segment is found
	// or iteration is finished,
	// mark it as true if one of the segments from this object is newer then specified threshold

	// will be used later
	// skip bool
}

// ObjectsMap map that keeps objects representation
type ObjectsMap map[Cluster]map[string]*Object

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
	defer writer.Flush()
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

	objects := make(ObjectsMap)
	inlineSegments := 0
	lastInlineSegments := 0
	remoteSegments := 0
	lastProjectID := ""

	// iterate method is serving entries in sorted order
	err = db.Iterate(ctx, storage.IterateOptions{Recurse: true},
		func(ctx context.Context, it storage.Iterator) error {
			var item storage.ListItem

			// iterate over every segment in metainfo
			for it.Next(ctx, &item) {
				rawPath := item.Key.String()
				pointer := &pb.Pointer{}

				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					return errs.New("unexpected error unmarshalling pointer %s", err)
				}

				streamMeta := pb.StreamMeta{}
				err = proto.Unmarshal(pointer.Metadata, &streamMeta)
				if err != nil {
					return errs.New("unexpected error unmarshalling pointer metadata %s", err)
				}

				pathElements := storj.SplitPath(rawPath)
				if len(pathElements) < 3 {
					return errs.New("invalid path %q", rawPath)
				}

				if len(pathElements) < 4 {
					return nil
				}

				cluster := Cluster{
					projectID: pathElements[0],
					bucket:    pathElements[2],
				}
				objectPath := storj.JoinPaths(storj.JoinPaths(pathElements[3:]...))
				object := findOrCreate(cluster, objectPath, objects)
				if lastProjectID != "" && lastProjectID != cluster.projectID {
					err = analyzeProject(ctx, db, objects, writer)
					if err != nil {
						return err
					}

					// cleanup map to free memory
					objects = make(ObjectsMap)
				}

				isLastSegment := pathElements[1] == "l"
				if isLastSegment {
					object.hasLastSegment = true
					if streamMeta.NumberOfSegments != 0 {
						if streamMeta.NumberOfSegments > int64(maxNumOfSegments) {
							return errs.New("unsupported number of segments: %d", streamMeta.NumberOfSegments)
						}
						object.expectedNumberOfSegments = byte(streamMeta.NumberOfSegments)
					}
				} else {
					segmentIndex, err := strconv.Atoi(pathElements[1][1:])
					if err != nil {
						return err
					}
					if segmentIndex >= int(maxNumOfSegments) {
						return errs.New("unsupported segment index: %d", segmentIndex)
					}

					if object.segments&(1<<uint64(segmentIndex)) != 0 {
						// TODO make path displayable
						return errs.New("fatal error this segment is duplicated: %s", rawPath)
					}

					object.segments |= 1 << uint64(segmentIndex)
				}

				// collect number of pointers for report
				if pointer.Type == pb.Pointer_INLINE {
					inlineSegments++
					if isLastSegment {
						lastInlineSegments++
					}
				} else {
					remoteSegments++
				}

				lastProjectID = cluster.projectID
			}
			return nil
		})
	log.Info("number of inline segments", zap.Int("segments", inlineSegments))
	log.Info("number of last inline segments", zap.Int("segments", lastInlineSegments))
	log.Info("number of remote segments", zap.Int("segments", remoteSegments))
	return nil
}

func analyzeProject(ctx context.Context, db metainfo.PointerDB, objectsMap ObjectsMap, csvWriter *csv.Writer) error {
	return nil
}

func findOrCreate(cluster Cluster, path string, objects ObjectsMap) *Object {
	var object *Object
	objectsMap, ok := objects[cluster]
	if !ok {
		objectsMap = make(map[string]*Object)
		objects[cluster] = objectsMap
	}

	object, ok = objectsMap[path]
	if !ok {
		object = &Object{}
		objectsMap[path] = object
	}

	return object
}
