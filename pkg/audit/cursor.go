// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storj"
)

// Stripe keeps track of a stripe's index and its parent segment
type Stripe struct {
	Index       int64
	Segment     *pb.Pointer
	SegmentPath storj.Path
}

// Cursor keeps track of audit location in pointer db
type Cursor struct {
	pointerdb *pointerdb.Service
	lastPath  storj.Path
	mutex     sync.Mutex
}

// NewCursor creates a Cursor which iterates over pointer db
func NewCursor(pointerdb *pointerdb.Service) *Cursor {
	return &Cursor{
		pointerdb: pointerdb,
	}
}

// NextStripe returns a random stripe to be audited
func (cursor *Cursor) NextStripe(ctx context.Context) (stripe *Stripe, err error) {
	cursor.mutex.Lock()
	defer cursor.mutex.Unlock()

	var pointerItems []*pb.ListResponse_Item
	var path storj.Path
	var more bool

	pointerItems, more, err = cursor.pointerdb.List("", cursor.lastPath, "", true, 0, meta.None)
	if err != nil {
		return nil, err
	}

	// keep track of last path listed
	if !more {
		cursor.lastPath = ""
	} else {
		cursor.lastPath = pointerItems[len(pointerItems)-1].Path
	}

	var pointer *pb.Pointer
	errGroup := new(errs.Group)

	pointer, err = cursor.getRandomValidPointer(pointerItems)
	if err != nil {
		return nil, err
	}

	index, err := getRandomStripe(pointer)
	if err != nil {
		return nil, err
	}

	return &Stripe{
		Index:       index,
		Segment:     pointer,
		SegmentPath: path,
	}, nil
}

func getRandomStripe(pointer *pb.Pointer) (index int64, err error) {
	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return 0, err
	}

	// the last segment could be smaller than stripe size
	if pointer.GetSegmentSize() < int64(redundancy.StripeSize()) {
		return 0, nil
	}

	numStripes := pointer.GetSegmentSize() / int64(redundancy.StripeSize())
	randomStripeIndex := rand.Int63n(numStripes)

	return randomStripeIndex, nil
}

// getRandomValidPointer attempts to get a random remote pointer from a list. If it sees expired pointers in the process of looking, deletes them
func (cursor *Cursor) getRandomValidPointer(pointerItems []*pb.ListResponse_Item) (pointer *pb.Pointer, err error) {
	if len(pointerItems) == 0 {
		return nil, Error.New("no stripes in pointerdb"))
	}

	errGroup := new(errs.Group)
	randomNums := rand.Perm(len(pointerItems))

	for _, randomIndex := range randomNums {
		pointerItem := pointerItems[randomIndex]
		path := pointerItem.Path

		// get pointer info
		pointer, err := cursor.pointerdb.Get(path)
		if err != nil {
			errGroup.Add(err)
			continue
		}

		//delete expired items rather than auditing them
		if expiration := pointer.GetExpirationDate(); expiration != nil {
			t, err := ptypes.Timestamp(expiration)
			if err != nil {
				errGroup.Add(err)
				continue
			}
			if t.Before(time.Now()) {
				err := cursor.pointerdb.Delete(path)
				if err != nil {
					errGroup.Add(err)
				}
				continue
			}
		}

		if pointer.GetType() != pb.Pointer_REMOTE || pointer.GetSegmentSize() == 0 {
			continue
		}

		return pointer, toDelete, nil
	}

	return nil, toDelete, errGroup.Err()
}
