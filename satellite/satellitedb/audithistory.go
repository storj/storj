// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"storj.io/common/pb"
	"storj.io/storj/satellite/reputation"
)

func updateAuditHistory(ctx context.Context, oldHistory []byte, config reputation.AuditHistoryConfig, online bool, auditTime time.Time) (res *reputation.UpdateAuditHistoryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	res = &reputation.UpdateAuditHistoryResponse{
		NewScore:           1,
		TrackingPeriodFull: false,
	}

	// deserialize node audit history
	history := &pb.AuditHistory{}
	err = pb.Unmarshal(oldHistory, history)
	if err != nil {
		return res, err
	}

	err = reputation.AddAuditToHistory(history, online, auditTime, config)
	if err != nil {
		return res, err
	}

	res.History, err = pb.Marshal(history)
	if err != nil {
		return res, err
	}

	windowsPerTrackingPeriod := int(config.TrackingPeriod.Seconds() / config.WindowSize.Seconds())
	res.TrackingPeriodFull = len(history.Windows)-1 >= windowsPerTrackingPeriod
	res.NewScore = history.Score
	return res, nil
}
