// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package objectdeletion

import (
	"context"

	"storj.io/common/pb"
)

// ObjectState determines how an object should be handled during
// a delete operation.
// It also stores pointers related to an object.
type ObjectState struct {
	ObjectIdentifier

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
func CreateObjectStates(ctx context.Context, requests []*ObjectIdentifier, pointers []*pb.Pointer, paths [][]byte) (map[string]*ObjectState, error) {

	// Fetch headers to figure out the status of objects.
	objects := make(map[string]*ObjectState)
	for _, req := range requests {
		objects[req.Key()] = &ObjectState{
			ObjectIdentifier: *req,
		}
	}

	for i, p := range paths {
		// Update our state map.
		id, segment, err := ParseSegmentPath(p)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		state, ok := objects[id.Key()]
		if !ok {
			return nil, Error.Wrap(err)
		}

		switch segment {
		case lastSegmentIndex:
			state.LastSegment = pointers[i]
		case firstSegmentIndex:
			state.ZeroSegment = pointers[i]
		default:
			return nil, Error.New("pointerDB failure")
		}
	}
	return objects, nil
}
