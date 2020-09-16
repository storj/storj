// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package objectdeletion

import (
	"context"

	"storj.io/common/pb"
	"storj.io/storj/satellite/metainfo/metabase"
)

// ObjectState determines how an object should be handled during
// a delete operation.
// It also stores pointers related to an object.
type ObjectState struct {
	metabase.ObjectLocation

	LastSegment *pb.Pointer
	ZeroSegment *pb.Pointer

	OtherSegments []*pb.Pointer
}

// Status returns the current object status.
func (state *ObjectState) Status() ObjectStatus {
	switch {
	case state.LastSegment == nil && state.ZeroSegment == nil:
		return ObjectMissing
	case state.LastSegment != nil && state.ZeroSegment == nil:
		return ObjectSingleSegment
	case state.LastSegment != nil && state.ZeroSegment != nil:
		return ObjectMultiSegment
	case state.LastSegment == nil && state.ZeroSegment != nil:
		return ObjectActiveOrZombie
	default:
		return ObjectMissing
	}
}

// ObjectStatus is used to determine how to retrieve segment information
// from the database.
type ObjectStatus byte

const (
	// ObjectMissing represents when there's no object with that particular name.
	ObjectMissing = ObjectStatus(iota)
	// ObjectMultiSegment represents a multi segment object.
	ObjectMultiSegment
	// ObjectSingleSegment represents a single segment object.
	ObjectSingleSegment
	// ObjectActiveOrZombie represents either an object is being uploaded, deleted or it's a zombie.
	ObjectActiveOrZombie
)

// CreateObjectStates creates the current object states.
func CreateObjectStates(ctx context.Context, requests []*metabase.ObjectLocation, pointers []*pb.Pointer, paths []metabase.SegmentKey) (map[metabase.ObjectLocation]*ObjectState, error) {

	// Fetch headers to figure out the status of objects.
	objects := make(map[metabase.ObjectLocation]*ObjectState)
	for _, req := range requests {
		objects[*req] = &ObjectState{
			ObjectLocation: *req,
		}
	}

	for i, p := range paths {
		// Update our state map.
		segmentLocation, err := metabase.ParseSegmentKey(p)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		state, ok := objects[segmentLocation.Object()]
		if !ok {
			return nil, Error.Wrap(err)
		}

		switch {
		case segmentLocation.IsLast():
			state.LastSegment = pointers[i]
		case segmentLocation.IsFirst():
			state.ZeroSegment = pointers[i]
		default:
			return nil, Error.New("pointerDB failure")
		}
	}
	return objects, nil
}
