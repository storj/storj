// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package avrometabase

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// SegmentFromRecord parses a segment from an Avro record map.
func SegmentFromRecord(ctx context.Context, recMap map[string]any, aliasCache *metabase.NodeAliasCache) (metabase.LoopSegmentEntry, error) {
	streamID, err := BytesToType(recMap["stream_id"], uuid.FromBytes)
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}

	positionEncoded, err := ToInt64(recMap["position"])
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}
	position := metabase.SegmentPositionFromEncoded(uint64(positionEncoded))

	createdAt, err := ToTime(recMap["created_at"])
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}

	expiresAt, err := ToTimeP(recMap["expires_at"])
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}

	repairedAt, err := ToTimeP(recMap["repaired_at"])
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}

	rootPieceID, err := BytesToType(recMap["root_piece_id"], storj.PieceIDFromBytes)
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}

	encryptedSize, err := ToInt64(recMap["encrypted_size"])
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}
	plainOffset, err := ToInt64(recMap["plain_offset"])
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}

	plainSize, err := ToInt64(recMap["plain_size"])
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}

	aliasPiecesBytes, err := ToBytes(recMap["remote_alias_pieces"])
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}
	var aliasPieces metabase.AliasPieces
	err = aliasPieces.SetBytes(aliasPiecesBytes)
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}

	redundancyInt64, err := ToInt64(recMap["redundancy"])
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}

	var redundancy storj.RedundancyScheme
	err = redundancy.Scan(redundancyInt64)
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}

	placement, err := ToInt64(recMap["placement"])
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}

	// TODO may think about memory optimization here
	pieces, err := aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
	if err != nil {
		return metabase.LoopSegmentEntry{}, errs.Wrap(err)
	}

	return metabase.LoopSegmentEntry{
		StreamID:      streamID,
		Position:      position,
		CreatedAt:     createdAt,
		ExpiresAt:     expiresAt,
		RepairedAt:    repairedAt,
		RootPieceID:   rootPieceID,
		EncryptedSize: int32(encryptedSize), // TODO type check
		PlainOffset:   plainOffset,
		PlainSize:     int32(plainSize), // TODO type check
		AliasPieces:   aliasPieces,
		Pieces:        pieces,
		Redundancy:    redundancy,
		Placement:     storj.PlacementConstraint(placement),
		Source:        "avro",
	}, nil
}

// ToInt64 converts an Avro value to int64.
func ToInt64(value any) (int64, error) {
	if value == nil {
		return 0, nil
	}

	switch value := value.(type) {
	case int64:
		return value, nil
	case map[string]any:
		return ToInt64(value["long"])
	default:
		return 0, errs.New("unable to cast type to int64: %T", value)
	}
}

// ToTime converts an Avro value to time.Time.
func ToTime(value any) (time.Time, error) {
	t, err := ToTimeP(value)
	if err != nil {
		return time.Time{}, err
	}
	if t == nil {
		return time.Time{}, nil
	}
	return *t, nil
}

// ToTimeP converts an Avro value to *time.Time.
func ToTimeP(value any) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	switch value := value.(type) {
	case string:
		if value == "" {
			return nil, nil
		}
		t, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, errs.New("failed to parse time: %v", err)
		}
		return &t, nil
	case map[string]any:
		return ToTimeP(value["string"])
	default:
		return nil, errs.New("unable to cast type to time.Time: %T", value)
	}
}

// ToBytes converts an Avro value to []byte.
func ToBytes(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}

	switch value := value.(type) {
	case []byte:
		return value, nil
	case map[string]any:
		return ToBytes(value["bytes"])
	default:
		return nil, errs.New("unable to cast type to []byte: %T", value)
	}
}

// BytesToType converts an Avro bytes value to a specific type using the provided conversion function.
func BytesToType[T any](value any, fn func([]byte) (T, error)) (result T, err error) {
	valueBytes, err := ToBytes(value)
	if err != nil {
		return result, err
	}
	return fn(valueBytes)
}
