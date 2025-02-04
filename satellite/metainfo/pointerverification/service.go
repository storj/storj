// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package pointerverification implements verification of pointers.
package pointerverification

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/trust"
)

var (
	mon = monkit.Package()
	// Error general pointer verification error.
	Error = errs.Class("pointer verification")
)

const pieceHashExpiration = 24 * time.Hour

// Service is a service for verifying validity of pieces.
type Service struct {
	identities     *IdentityCache
	overlay        *overlay.Service
	trustedUplinks *trust.TrustedPeersList
	trustedOrders  bool
}

// NewService returns a service using the provided database.
func NewService(db overlay.PeerIdentities, overlay *overlay.Service, trustedUplinks *trust.TrustedPeersList, trustedOrders bool) *Service {
	return &Service{
		identities:     NewIdentityCache(db),
		overlay:        overlay,
		trustedUplinks: trustedUplinks,
		trustedOrders:  trustedOrders,
	}
}

// VerifySizes verifies that the remote piece sizes in commitSegment match each other.
func (service *Service) VerifySizes(ctx context.Context, redundancy storj.RedundancyScheme, encryptedSize int64, uploadResponses []*pb.SegmentPieceUploadResult) (err error) {
	defer mon.Task()(&ctx)(&err)

	commonSize := int64(-1)
	for _, piece := range uploadResponses {
		if piece.Hash == nil {
			continue
		}
		if piece.Hash.PieceSize <= 0 {
			return Error.New("size is invalid (%d)", piece.Hash.PieceSize)
		}

		if commonSize > 0 && commonSize != piece.Hash.PieceSize {
			return Error.New("sizes do not match (%d != %d)", commonSize, piece.Hash.PieceSize)
		}

		commonSize = piece.Hash.PieceSize
	}

	if commonSize < 0 {
		return Error.New("no remote pieces")
	}

	expectedSize := redundancy.PieceSize(encryptedSize)
	if expectedSize != commonSize {
		return Error.New("expected size is different from provided (%d != %d)", expectedSize, commonSize)
	}

	return nil
}

// InvalidPiece is information about an invalid piece in the pointer.
type InvalidPiece struct {
	NodeID   storj.NodeID
	PieceNum int32
	Reason   error
}

// SelectValidPieces selects pieces that are have correct hashes and match the original limits.
func (service *Service) SelectValidPieces(ctx context.Context, uplink *identity.PeerIdentity, uploadResponses []*pb.SegmentPieceUploadResult, originalLimits []*pb.OrderLimit) (valid []*pb.SegmentPieceUploadResult, invalid []InvalidPiece, err error) {
	defer mon.Task()(&ctx)(&err)

	trustedOrders := service.trustedOrders && service.trustedUplinks.IsTrusted(uplink.ID)
	untrustedNodeIDs := make([]storj.NodeID, 0, len(uploadResponses))
	nodeTrustedByUploadResponse := make([]bool, len(uploadResponses))

	for uploadResponseIdx, piece := range uploadResponses {
		if trustedOrders {
			if node, err := service.overlay.CachedGet(ctx, piece.NodeId); err == nil {
				if tag, err := node.Tags.FindBySignerAndName(trust.TrustedOperatorSigner, "trusted_orders"); err == nil {
					if string(tag.Value) == "true" {
						nodeTrustedByUploadResponse[uploadResponseIdx] = true
						continue
					}
				}
			}
		}
		untrustedNodeIDs = append(untrustedNodeIDs, piece.NodeId)
	}

	if len(untrustedNodeIDs) > 0 {
		err = service.identities.EnsureCached(ctx, untrustedNodeIDs)
		if err != nil {
			return nil, nil, Error.Wrap(err)
		}
	}

	alreadyUsedPieceNum := map[int32]bool{}

	for uploadResponseIdx, piece := range uploadResponses {
		if int(piece.PieceNum) >= len(originalLimits) {
			invalid = append(invalid, InvalidPiece{
				NodeID:   piece.NodeId,
				PieceNum: piece.PieceNum,
				Reason:   Error.New("invalid piece number"),
			})
			continue
		}

		limit := originalLimits[piece.PieceNum]
		if limit == nil {
			invalid = append(invalid, InvalidPiece{
				NodeID:   piece.NodeId,
				PieceNum: piece.PieceNum,
				Reason:   Error.New("limit missing for piece"),
			})
			continue
		}

		// verify that the piece id, serial number etc. match in piece and limit.
		if err := VerifyPieceAndLimit(ctx, piece, limit); err != nil {
			invalid = append(invalid, InvalidPiece{
				NodeID:   piece.NodeId,
				PieceNum: piece.PieceNum,
				Reason:   err,
			})
			continue
		}

		if !nodeTrustedByUploadResponse[uploadResponseIdx] {
			signee := service.identities.GetCached(ctx, piece.NodeId)
			if signee == nil {
				// This shouldn't happen due to the caching in the start of the func.
				return nil, nil, Error.New("nil identity returned (%v)", piece.NodeId)
			}

			// verify the signature
			err = signing.VerifyPieceHashSignature(ctx, signee, piece.Hash)
			if err != nil {
				// TODO: check whether the identity changed from what it was before.

				// Maybe the cache has gone stale?
				signee, err := service.identities.GetUpdated(ctx, piece.NodeId)
				if err != nil {
					return nil, nil, Error.Wrap(err)
				}

				// let's check the signature again
				err = signing.VerifyPieceHashSignature(ctx, signee, piece.Hash)
				if err != nil {
					invalid = append(invalid, InvalidPiece{
						NodeID:   piece.NodeId,
						PieceNum: piece.PieceNum,
						Reason:   err,
					})
					continue
				}
			}
		}

		if alreadyUsedPieceNum[piece.PieceNum] {
			invalid = append(invalid, InvalidPiece{
				NodeID:   piece.NodeId,
				PieceNum: piece.PieceNum,
				Reason:   Error.New("duplicate piece number"),
			})
			continue
		}
		alreadyUsedPieceNum[piece.PieceNum] = true

		valid = append(valid, piece)
	}

	return valid, invalid, nil
}

// VerifyPieceAndLimit verifies that the piece and limit match.
func VerifyPieceAndLimit(ctx context.Context, piece *pb.SegmentPieceUploadResult, limit *pb.OrderLimit) (err error) {
	defer mon.Task()(&ctx)(&err)

	// ensure that we have a hash
	if piece.Hash == nil {
		return Error.New("no piece hash. NodeID: %v, PieceNum: %d", piece.NodeId, piece.PieceNum)
	}

	// verify the timestamp
	timestamp := piece.Hash.Timestamp
	if timestamp.Before(time.Now().Add(-pieceHashExpiration)) {
		return Error.New("piece hash timestamp is too old (%v). NodeId: %v, PieceNum: %d)",
			timestamp, piece.NodeId, piece.PieceNum,
		)
	}

	// verify the piece id
	if limit.PieceId != piece.Hash.PieceId {
		return Error.New("piece hash pieceID (%v) doesn't match limit pieceID (%v). NodeID: %v, PieceNum: %d",
			piece.Hash.PieceId, limit.PieceId, piece.NodeId, piece.PieceNum,
		)
	}

	// verify the limit
	if limit.Limit < piece.Hash.PieceSize {
		return Error.New("piece hash PieceSize (%d) is larger than order limit (%d). NodeID: %v, PieceNum: %d",
			piece.Hash.PieceSize, limit.Limit, piece.NodeId, piece.PieceNum,
		)
	}

	return nil
}
