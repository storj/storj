// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import "storj.io/common/pb"

func pieceHashAndOrderLimitFromReader(reader PieceReader) (_ pb.PieceHash, _ pb.OrderLimit, err error) {
	header, err := reader.GetPieceHeader()
	if err != nil {
		return pb.PieceHash{}, pb.OrderLimit{}, err
	}
	return pb.PieceHash{
		PieceId:       header.GetOrderLimit().PieceId,
		Hash:          header.GetHash(),
		HashAlgorithm: header.GetHashAlgorithm(),
		PieceSize:     reader.Size(),
		Timestamp:     header.GetCreationTime(),
		Signature:     header.GetSignature(),
	}, header.OrderLimit, nil
}
