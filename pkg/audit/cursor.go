// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"crypto/rand"
	"math/big"
	"sync"

	"github.com/golang/protobuf/ptypes/timestamp"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storj"
)

// Stripe keeps track of a stripe's index and its parent segment
type Stripe struct {
	Index       int64
	Segment     *pb.Pointer
	OrderLimits []*pb.AddressedOrderLimit
}

// Cursor keeps track of audit location in pointer db
type Cursor struct {
	pointerdb  *pointerdb.Service
	allocation *pointerdb.AllocationSigner
	cache      *overlay.Cache
	identity   *identity.FullIdentity
	signer     signing.Signer
	lastPath   storj.Path
	mutex      sync.Mutex
}

// NewCursor creates a Cursor which iterates over pointer db
func NewCursor(pointerdb *pointerdb.Service, allocation *pointerdb.AllocationSigner, cache *overlay.Cache, identity *identity.FullIdentity) *Cursor {
	return &Cursor{
		pointerdb:  pointerdb,
		allocation: allocation,
		cache:      cache,
		identity:   identity,
		signer:     signing.SignerFromFullIdentity(identity),
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
	pointer, err := cursor.pointerdb.Get(path)
	if err != nil {
		return nil, err
	}

	if pointer.GetType() != pb.Pointer_REMOTE {
		return nil, nil
	}

	if pointer.GetSegmentSize() == 0 {
		return nil, nil
	}

	index, err := getRandomStripe(pointer)
	if err != nil {
		return nil, err
	}

	limits, err := cursor.createOrderLimits(ctx, pointer)
	if err != nil {
		return nil, err
	}

	return &Stripe{
		Index:       index,
		Segment:     pointer,
		OrderLimits: limits,
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

func getRandomPointer(pointerItems []*pb.ListResponse_Item) (pointer *pb.ListResponse_Item, err error) {
	randomNum, err := rand.Int(rand.Reader, big.NewInt(int64(len(pointerItems))))
	if err != nil {
		return &pb.ListResponse_Item{}, err
	}

	return pointerItems[randomNum.Int64()], nil
}

func (cursor *Cursor) createOrderLimits(ctx context.Context, pointer *pb.Pointer) ([]*pb.AddressedOrderLimit, error) {
	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return nil, err
	}

	auditorIdentity := cursor.identity.PeerIdentity()
	rootPieceID := pointer.GetRemote().RootPieceId
	pieceSize := eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy)
	expiration := pointer.ExpirationDate

	limits := make([]*pb.AddressedOrderLimit, redundancy.TotalCount())
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		derivedPieceID := rootPieceID.Derive(piece.NodeId)
		orderLimit, err := cursor.createOrderLimit(ctx, auditorIdentity, piece.NodeId, derivedPieceID, expiration, pieceSize, pb.Action_GET_AUDIT)
		if err != nil {
			return nil, err
		}

		node, err := cursor.cache.Get(ctx, piece.NodeId)
		if err != nil {
			return nil, err
		}

		if node != nil {
			node.Type.DPanicOnInvalid("auditor order limits")
		}

		limits[piece.GetPieceNum()] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
	}

	return limits, nil
}

func (cursor *Cursor) createOrderLimit(ctx context.Context, uplinkIdentity *identity.PeerIdentity, nodeID storj.NodeID, pieceID storj.PieceID, expiration *timestamp.Timestamp, limit int64, action pb.Action) (*pb.OrderLimit2, error) {
	parameters := pointerdb.OrderLimitParameters{
		UplinkIdentity:  uplinkIdentity,
		StorageNodeID:   nodeID,
		PieceID:         pieceID,
		Action:          action,
		PieceExpiration: expiration,
		Limit:           limit,
	}

	orderLimit, err := cursor.allocation.OrderLimit(ctx, parameters)
	if err != nil {
		return nil, err
	}

	orderLimit, err = signing.SignOrderLimit(cursor.signer, orderLimit)
	if err != nil {
		return nil, err
	}

	return orderLimit, nil
}
