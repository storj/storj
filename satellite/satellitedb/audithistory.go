// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/common/pb"
	"storj.io/storj/satellite/reputation"
)

func mergeAuditHistory(ctx context.Context, oldHistory []byte, addHistory []*pb.AuditWindow, config reputation.AuditHistoryConfig) (res *reputation.UpdateAuditHistoryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	history := &pb.AuditHistory{}
	err = pb.Unmarshal(oldHistory, history)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	trackingPeriodFull := reputation.MergeAuditHistories(history, addHistory, config)

	historyBytes, err := pb.Marshal(history)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &reputation.UpdateAuditHistoryResponse{
		NewScore:           history.Score,
		TrackingPeriodFull: trackingPeriodFull,
		History:            historyBytes,
	}, nil
}
