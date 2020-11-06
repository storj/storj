// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"strconv"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// Error is the default error for metabase.
var Error = errs.Class("metabase")

// Common constants for segment keys.
const (
	LastSegmentName   = "l"
	LastSegmentIndex  = -1
	FirstSegmentIndex = 0
)

// BucketPrefix consists of <project id>/<bucket name>.
type BucketPrefix string

// BucketLocation defines a bucket that belongs to a project.
type BucketLocation struct {
	ProjectID  uuid.UUID
	BucketName string
}

// ParseBucketPrefix parses BucketPrefix.
func ParseBucketPrefix(prefix BucketPrefix) (BucketLocation, error) {
	elements := strings.Split(string(prefix), "/")
	if len(elements) != 2 {
		return BucketLocation{}, Error.New("invalid prefix %q", prefix)
	}

	projectID, err := uuid.FromString(elements[0])
	if err != nil {
		return BucketLocation{}, Error.Wrap(err)
	}

	return BucketLocation{
		ProjectID:  projectID,
		BucketName: elements[1],
	}, nil
}

// Prefix converts bucket location into bucket prefix.
func (loc BucketLocation) Prefix() BucketPrefix {
	return BucketPrefix(loc.ProjectID.String() + "/" + loc.BucketName)
}

// ObjectKey is an encrypted object key encoded using Path Component Encoding.
// It is not ascii safe.
type ObjectKey string

// ObjectLocation is decoded object key information.
type ObjectLocation struct {
	ProjectID  uuid.UUID
	BucketName string
	ObjectKey  ObjectKey
}

// Bucket returns bucket location this object belongs to.
func (obj ObjectLocation) Bucket() BucketLocation {
	return BucketLocation{
		ProjectID:  obj.ProjectID,
		BucketName: obj.BucketName,
	}
}

// LastSegment returns the last segment location.
func (obj ObjectLocation) LastSegment() SegmentLocation {
	return SegmentLocation{
		ProjectID:  obj.ProjectID,
		BucketName: obj.BucketName,
		Index:      LastSegmentIndex,
		ObjectKey:  obj.ObjectKey,
	}
}

// FirstSegment returns the first segment location.
func (obj ObjectLocation) FirstSegment() SegmentLocation {
	return SegmentLocation{
		ProjectID:  obj.ProjectID,
		BucketName: obj.BucketName,
		Index:      FirstSegmentIndex,
		ObjectKey:  obj.ObjectKey,
	}
}

// Segment returns segment location for a given index.
func (obj ObjectLocation) Segment(index int64) (SegmentLocation, error) {
	if index < LastSegmentIndex {
		return SegmentLocation{}, Error.New("invalid index %v", index)
	}
	return SegmentLocation{
		ProjectID:  obj.ProjectID,
		BucketName: obj.BucketName,
		Index:      index,
		ObjectKey:  obj.ObjectKey,
	}, nil
}

// SegmentKey is an encoded metainfo key. This is used as the key in pointerdb key-value store.
type SegmentKey []byte

// SegmentLocation is decoded segment key information.
type SegmentLocation struct {
	ProjectID  uuid.UUID
	BucketName string
	Index      int64
	ObjectKey  ObjectKey
}

// Bucket returns bucket location this segment belongs to.
func (seg SegmentLocation) Bucket() BucketLocation {
	return BucketLocation{
		ProjectID:  seg.ProjectID,
		BucketName: seg.BucketName,
	}
}

// Object returns the object location associated with this segment location.
func (seg SegmentLocation) Object() ObjectLocation {
	return ObjectLocation{
		ProjectID:  seg.ProjectID,
		BucketName: seg.BucketName,
		ObjectKey:  seg.ObjectKey,
	}
}

// IsLast returns whether this corresponds to last segment.
func (seg SegmentLocation) IsLast() bool { return seg.Index == LastSegmentIndex }

// IsFirst returns whether this corresponds to first segment.
func (seg SegmentLocation) IsFirst() bool { return seg.Index == FirstSegmentIndex }

// ParseSegmentKey parses an segment key into segment location.
func ParseSegmentKey(encoded SegmentKey) (SegmentLocation, error) {
	elements := strings.SplitN(string(encoded), "/", 4)
	if len(elements) < 4 {
		return SegmentLocation{}, Error.New("invalid key %q", encoded)
	}

	projectID, err := uuid.FromString(elements[0])
	if err != nil {
		return SegmentLocation{}, Error.New("invalid key %q", encoded)
	}

	var index int64
	if elements[1] == LastSegmentName {
		index = LastSegmentIndex
	} else {
		numstr := strings.TrimPrefix(elements[1], "s")
		// remove prefix `s` from segment index we got
		index, err = strconv.ParseInt(numstr, 10, 64)
		if err != nil {
			return SegmentLocation{}, Error.New("invalid %q, segment number %q", string(encoded), elements[1])
		}
	}

	return SegmentLocation{
		ProjectID:  projectID,
		BucketName: elements[2],
		Index:      index,
		ObjectKey:  ObjectKey(elements[3]),
	}, nil
}

// Encode converts segment location into a segment key.
func (seg SegmentLocation) Encode() SegmentKey {
	segment := LastSegmentName
	if seg.Index > LastSegmentIndex {
		segment = "s" + strconv.FormatInt(seg.Index, 10)
	}
	return SegmentKey(storj.JoinPaths(
		seg.ProjectID.String(),
		segment,
		seg.BucketName,
		string(seg.ObjectKey),
	))
}

// Pieces defines information for pieces.
type Pieces []Piece

// Piece defines information for a segment piece.
type Piece struct {
	Number      uint16
	StorageNode storj.NodeID
}
