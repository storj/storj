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
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/skyrings/skyring-common/tools/uuid"

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

	//delete expired items rather than auditing them
	if expiration := pointer.GetExpirationDate(); expiration != nil {
		t, err := ptypes.Timestamp(expiration)
		if err != nil {
			return nil, err
		}
		if t.Before(time.Now()) {
			return nil, cursor.pointerdb.Delete(path)
		}
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
	auditorIdentity := cursor.identity.PeerIdentity()
	rootPieceID := pointer.GetRemote().RootPieceId
	shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()
	expiration := pointer.ExpirationDate

	// Add Serial Number for the entire pointer audit
	// needs to be the same for all nodes in the pointer
	uuid, err := uuid.New()
	if err != nil {
		return nil, err
	}
	serialNumber := storj.SerialNumber(*uuid)

	limits := make([]*pb.AddressedOrderLimit, pointer.GetRemote().GetRedundancy().GetTotal())
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		derivedPieceID := rootPieceID.Derive(piece.NodeId)
		orderLimit, err := cursor.createOrderLimit(ctx, serialNumber, auditorIdentity, piece.NodeId, derivedPieceID, expiration, int64(shareSize), pb.PieceAction_GET_AUDIT)
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

	//ToDo: We have to save the orderLimit here into the satellitedb
	/*if err := endpoint.saveRemoteOrder(ctx, keyInfo.ProjectID, req.Bucket, addressedLimits); err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}*/

	return limits, nil
}

func (cursor *Cursor) createOrderLimit(ctx context.Context, serialnumber storj.SerialNumber, uplinkIdentity *identity.PeerIdentity, nodeID storj.NodeID, pieceID storj.PieceID, expiration *timestamp.Timestamp, limit int64, action pb.PieceAction) (*pb.OrderLimit2, error) {

	parameters := pointerdb.OrderLimitParameters{
		SerialNumber:    serialnumber,
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
