// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
)

// Stripe keeps track of a stripe's index and its parent segment
type Stripe struct {
	Index       int64
	Segment     *pb.Pointer
	SegmentPath storj.Path
}

// Cursor keeps track of audit location in pointer db
type Cursor struct {
	metainfo *metainfo.Service
	lastPath storj.Path
	mutex    sync.Mutex
}

// NewCursor creates a Cursor which iterates over pointer db
func NewCursor(metainfo *metainfo.Service) *Cursor {
	return &Cursor{
		metainfo: metainfo,
	}
}

// NextStripe returns a random stripe to be audited. "more" is true except when we have completed iterating over metainfo. It can be disregarded if there is an error or stripe returned
func (cursor *Cursor) NextStripe(ctx context.Context) (stripe *Stripe, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	cursor.mutex.Lock()
	defer cursor.mutex.Unlock()

	var pointerItems []*pb.ListResponse_Item
	var path storj.Path

	pointerItems, more, err = cursor.metainfo.List(ctx, "", cursor.lastPath, "", true, 0, meta.None)
	if err != nil {
		return nil, more, err
	}

	// keep track of last path listed
	if !more {
		cursor.lastPath = ""
	} else {
		cursor.lastPath = pointerItems[len(pointerItems)-1].Path
	}

	pointer, path, err := cursor.getRandomValidPointer(ctx, pointerItems)
	if err != nil {
		return nil, more, err
	}
	if pointer == nil {
		return nil, more, nil
	}

	index, err := getRandomStripe(ctx, pointer)
	if err != nil {
		return nil, more, err
	}

	return &Stripe{
		Index:       index,
		Segment:     pointer,
		SegmentPath: path,
	}, more, nil
}

func getRandomStripe(ctx context.Context, pointer *pb.Pointer) (index int64, err error) {
	defer mon.Task()(&ctx)(&err)
	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return 0, err
	}

	// the last segment could be smaller than stripe size
	if pointer.GetSegmentSize() < int64(redundancy.StripeSize()) {
		return 0, nil
	}

	var src cryptoSource
	rnd := rand.New(src)
	numStripes := pointer.GetSegmentSize() / int64(redundancy.StripeSize())
	randomStripeIndex := rnd.Int63n(numStripes)

	return randomStripeIndex, nil
}

// getRandomValidPointer attempts to get a random remote pointer from a list. If it sees expired pointers in the process of looking, deletes them
func (cursor *Cursor) getRandomValidPointer(ctx context.Context, pointerItems []*pb.ListResponse_Item) (pointer *pb.Pointer, path storj.Path, err error) {
	defer mon.Task()(&ctx)(&err)
	var src cryptoSource
	rnd := rand.New(src)
	errGroup := new(errs.Group)
	randomNums := rnd.Perm(len(pointerItems))

	for _, randomIndex := range randomNums {
		pointerItem := pointerItems[randomIndex]
		path := pointerItem.Path

		// get pointer info
		pointer, err := cursor.metainfo.Get(ctx, path)
		if err != nil {
			errGroup.Add(err)
			continue
		}

		//delete expired items rather than auditing them
		if !pointer.ExpirationDate.IsZero() && pointer.ExpirationDate.Before(time.Now()) {
			err := cursor.metainfo.Delete(ctx, path)
			if err != nil {
				errGroup.Add(err)
			}
			continue
		}

		if pointer.GetType() != pb.Pointer_REMOTE || pointer.GetSegmentSize() == 0 {
			continue
		}

		return pointer, path, nil
	}

	return nil, "", errGroup.Err()
}

// cryptoSource implements the math/rand Source interface using crypto/rand
type cryptoSource struct{}

func (s cryptoSource) Seed(seed int64) {}

func (s cryptoSource) Int63() int64 {
	return int64(s.Uint64() & ^uint64(1<<63))
}

func (s cryptoSource) Uint64() (v uint64) {
	err := binary.Read(crand.Reader, binary.BigEndian, &v)
	if err != nil {
		panic(err)
	}
	return v
}
