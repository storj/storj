// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"crypto/rand"
	"math/big"
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
	for {
		if len(pointerItems) == 0 {
			errGroup.Add(Error.New("no stripes in pointerdb"))
			return nil, errGroup.Err()
		}

		pointer, pointerItems, err = cursor.getRandomPointer(pointerItems)
		if err != nil {
			errGroup.Add(err)
		}
		if pointer != nil {
			break
		}
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

	randomStripeIndex, err := rand.Int(rand.Reader, big.NewInt(pointer.GetSegmentSize()/int64(redundancy.StripeSize())))
	if err != nil {
		return -1, err
	}

	return randomStripeIndex.Int64(), nil
}

// getRandomPointer attempts to get a random pointer from a list, and if the selected pointer is invalid, it returns a list without that item
func (cursor *Cursor) getRandomPointer(pointerItems []*pb.ListResponse_Item) (pointer *pb.Pointer, newPointerItems []*pb.ListResponse_Item, err error) {
	randomNum, err := rand.Int(rand.Reader, big.NewInt(int64(len(pointerItems))))
	if err != nil {
		return nil, pointerItems, err
	}

	i := randomNum.Int64()
	pointerItem := pointerItems[i]
	path := pointerItem.Path

	// get pointer info
	pointer, err = cursor.pointerdb.Get(path)
	if err != nil {
		return nil, pointerItems, err
	}

	newPointerItems = append(pointerItems[:i], pointerItems[i+1:]...)

	//delete expired items rather than auditing them
	if expiration := pointer.GetExpirationDate(); expiration != nil {
		t, err := ptypes.Timestamp(expiration)
		if err != nil {
			return nil, newPointerItems, err
		}
		if t.Before(time.Now()) {
			return nil, newPointerItems, cursor.pointerdb.Delete(path)
		}
	}

	if pointer.GetType() != pb.Pointer_REMOTE {
		return nil, newPointerItems, Error.New("not remote pointer")
	}

	if pointer.GetSegmentSize() == 0 {
		return nil, newPointerItems, Error.New("segment size 0")
	}

	return pointer, newPointerItems, nil
}
