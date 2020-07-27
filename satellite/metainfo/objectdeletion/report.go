// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package objectdeletion

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/pb"
)

// Report represents the deleteion status report.
type Report struct {
	log *zap.Logger

	Deleted []*ObjectState
	Failed  []*ObjectState
}

// DeletedPointers returns all deleted pointers in a report.
func (r Report) DeletedPointers() []*pb.Pointer {
	pointers := make([]*pb.Pointer, 0, len(r.Deleted))
	for _, d := range r.Deleted {
		pointers = append(pointers, d.LastSegment)
		pointers = append(pointers, d.OtherSegments...)
	}

	return pointers
}

// HasFailures returns wether a delete operation has failures.
func (r Report) HasFailures() bool {
	return len(r.Failed) > 0
}

// DeletedObjects returns successfully deleted objects information.
func (r Report) DeletedObjects() []*pb.Object {
	objects := make([]*pb.Object, 0, len(r.Deleted))
	for _, d := range r.Deleted {
		if d.LastSegment == nil {
			continue
		}
		streamMeta := &pb.StreamMeta{}
		err := pb.Unmarshal(d.LastSegment.Metadata, streamMeta)
		if err != nil {
			r.log.Error("failed to unmarshal stream metadata",
				zap.Stringer("Project ID", d.ProjectID),
				zap.String("Bucket", string(d.Bucket)),
				zap.String("Encrypted Path", string(d.EncryptedPath)),
				zap.Error(err),
			)
			continue
		}
		object := &pb.Object{
			Bucket:            d.Bucket,
			EncryptedPath:     d.EncryptedPath,
			Version:           -1,
			ExpiresAt:         d.LastSegment.ExpirationDate,
			CreatedAt:         d.LastSegment.CreationDate,
			EncryptedMetadata: d.LastSegment.Metadata,
			EncryptionParameters: &pb.EncryptionParameters{
				CipherSuite: pb.CipherSuite(streamMeta.EncryptionType),
				BlockSize:   int64(streamMeta.EncryptionBlockSize),
			},
		}

		if d.LastSegment.Remote != nil {
			object.RedundancyScheme = d.LastSegment.Remote.Redundancy

		} else if streamMeta.NumberOfSegments == 0 || streamMeta.NumberOfSegments > 1 {
			// workaround
			// NumberOfSegments == 0 - pointer with encrypted num of segments
			// NumberOfSegments > 1 - pointer with unencrypted num of segments and multiple segments
			// The new metainfo API redundancy scheme is on object level (not per segment).
			// Because of that, RS is always taken from the last segment.
			// The old implementation saves RS per segment, and in some cases
			// when the remote file's last segment is an inline segment, we end up
			// missing an RS scheme. This loop will search for RS in segments other than the last one.

			for _, pointer := range d.OtherSegments {
				if pointer.Remote != nil {
					object.RedundancyScheme = pointer.Remote.Redundancy
					break
				}
			}
		}

		objects = append(objects, object)
	}

	return objects
}

// GenerateReport returns the result of a delete, success, or failure.
func GenerateReport(ctx context.Context, log *zap.Logger, requests []*ObjectIdentifier, deletedPaths [][]byte, pointers []*pb.Pointer) Report {
	defer mon.Task()(&ctx)(nil)

	report := Report{log: log}
	deletedObjects := make(map[string]*ObjectState)
	for i, path := range deletedPaths {
		if path == nil {
			continue
		}
		id, segmentIdx, err := ParseSegmentPath(path)
		if err != nil {
			log.Debug("failed to parse deleted segmnt path for report",
				zap.String("Raw Segment Path", string(path)),
			)
			continue
		}

		if _, ok := deletedObjects[id.Key()]; !ok {
			deletedObjects[id.Key()] = &ObjectState{
				OtherSegments: []*pb.Pointer{},
			}
		}

		switch segmentIdx {
		case lastSegmentIndex:
			deletedObjects[id.Key()].LastSegment = pointers[i]
		default:
			deletedObjects[id.Key()].OtherSegments = append(deletedObjects[id.Key()].OtherSegments, pointers[i])
		}
	}

	// populate report with failed and deleted objects
	for _, req := range requests {
		state, ok := deletedObjects[req.Key()]
		if !ok {
			report.Failed = append(report.Failed, &ObjectState{
				ObjectIdentifier: *req,
			})
			continue
		}

		state.ObjectIdentifier = *req
		report.Deleted = append(report.Deleted, state)
	}
	return report
}
