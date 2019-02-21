// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"crypto/rand"
	"math/big"
	"sync"

	"github.com/vivint/infectious"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storj"
)

// Stripe keeps track of a stripe's index and its parent segment
type Stripe struct {
	Index   int
	Segment *pb.Pointer
	PBA     *pb.PayerBandwidthAllocation
}

// Cursor keeps track of audit location in pointer db
type Cursor struct {
	pointers   *pointerdb.Service
	allocation *pointerdb.AllocationSigner
	identity   *identity.FullIdentity
	lastPath   storj.Path
	mutex      sync.Mutex
}

// NewCursor creates a Cursor which iterates over pointer db
func NewCursor(pointers *pointerdb.Service, allocation *pointerdb.AllocationSigner, identity *identity.FullIdentity) *Cursor {
	return &Cursor{pointers: pointers, allocation: allocation, identity: identity}
}

// NextStripe returns a random stripe to be audited
func (cursor *Cursor) NextStripe(ctx context.Context) (stripe *Stripe, err error) {
	cursor.mutex.Lock()
	defer cursor.mutex.Unlock()

	var pointerItems []*pb.ListResponse_Item
	var path storj.Path
	var more bool

	pointerItems, more, err = cursor.pointers.List("", cursor.lastPath, "", true, 0, meta.None)
	if err != nil {
		return nil, err
	}

	if len(pointerItems) == 0 {
		return nil, nil
	}

	pointerItem, err := getRandomPointer(pointerItems)
	if err != nil {
		return nil, err
	}

	path = pointerItem.Path

	// keep track of last path listed
	if !more {
		cursor.lastPath = ""
	} else {
		cursor.lastPath = pointerItems[len(pointerItems)-1].Path
	}

	// get pointer info
	pointer, err := cursor.pointers.Get(path)
	if err != nil {
		return nil, err
	}
	peerIdentity := &identity.PeerIdentity{ID: cursor.identity.ID, Leaf: cursor.identity.Leaf}
	pba, err := cursor.allocation.PayerBandwidthAllocation(ctx, peerIdentity, pb.BandwidthAction_GET_AUDIT)
	if err != nil {
		return nil, err
	}

	if pointer.GetType() != pb.Pointer_REMOTE {
		return nil, nil
	}

	// create the erasure scheme so we can get the stripe size
	es, err := makeErasureScheme(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return nil, err
	}

	if pointer.GetSegmentSize() == 0 {
		return nil, nil
	}

	index, err := getRandomStripe(es, pointer)
	if err != nil {
		return nil, err
	}

	return &Stripe{
		Index:   index,
		Segment: pointer,
		PBA:     pba,
	}, nil
}

func makeErasureScheme(rs *pb.RedundancyScheme) (eestream.ErasureScheme, error) {
	required := int(rs.GetMinReq())
	total := int(rs.GetTotal())

	fc, err := infectious.NewFEC(required, total)
	if err != nil {
		return nil, err
	}
	es := eestream.NewRSScheme(fc, int(rs.GetErasureShareSize()))
	return es, nil
}

func getRandomStripe(es eestream.ErasureScheme, pointer *pb.Pointer) (index int, err error) {
	stripeSize := es.StripeSize()

	// the last segment could be smaller than stripe size
	if pointer.GetSegmentSize() < int64(stripeSize) {
		return 0, nil
	}

	randomStripeIndex, err := rand.Int(rand.Reader, big.NewInt(pointer.GetSegmentSize()/int64(stripeSize)))
	if err != nil {
		return -1, err
	}
	return int(randomStripeIndex.Int64()), nil
}

func getRandomPointer(pointerItems []*pb.ListResponse_Item) (pointer *pb.ListResponse_Item, err error) {
	randomNum, err := rand.Int(rand.Reader, big.NewInt(int64(len(pointerItems))))
	if err != nil {
		return &pb.ListResponse_Item{}, err
	}
	randomNumInt64 := randomNum.Int64()
	pointerItem := pointerItems[randomNumInt64]
	return pointerItem, nil
}
