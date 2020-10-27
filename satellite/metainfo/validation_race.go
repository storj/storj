// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// +build race

package metainfo

import (
	"bytes"

	"storj.io/common/pb"
	"storj.io/storj/satellite/metainfo/metabase"
)

// sanityCheckPointer implements sanity checking test data,
// we don't need this in production code.
func sanityCheckPointer(key metabase.SegmentKey, pointer *pb.Pointer) (err error) {
	tokens := bytes.Split(key, []byte("/"))
	if len(tokens) <= 3 {
		return Error.New("invalid path %s", key)
	}

	if pointer.Type == pb.Pointer_REMOTE {
		remote := pointer.Remote

		switch {
		case remote.RootPieceId.IsZero():
			return Error.New("piece id zero")
		case remote == nil:
			return Error.New("no remote segment specified")
		case remote.RemotePieces == nil:
			return Error.New("no remote segment pieces specified")
		case remote.Redundancy == nil:
			return Error.New("no redundancy scheme specified")
		}

		redundancy := remote.Redundancy
		if redundancy.MinReq <= 0 || redundancy.Total <= 0 ||
			redundancy.RepairThreshold <= 0 || redundancy.SuccessThreshold <= 0 ||
			redundancy.ErasureShareSize <= 0 {
			return Error.New("invalid redundancy: %+v", redundancy)
		}

		for _, piece := range remote.GetRemotePieces() {
			if int(piece.PieceNum) >= int(redundancy.Total) {
				return Error.New("invalid PieceNum=%v total=%v", piece.PieceNum, redundancy.Total)
			}
		}
	}

	return nil
}
