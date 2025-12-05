// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/pb"
)

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

type timer struct {
	dv    *monkit.DurationVal
	start time.Time
	once  sync.Once
}

func newTimer(dv *monkit.DurationVal) *timer {
	return &timer{
		dv:    dv,
		start: time.Now(),
	}
}

func (t *timer) Trigger() {
	t.once.Do(func() {
		t.dv.Observe(time.Since(t.start))
	})
}
