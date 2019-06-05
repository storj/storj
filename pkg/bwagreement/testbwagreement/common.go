// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testbwagreement

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var mon = monkit.Package()

//GenerateOrderLimit creates a signed OrderLimit from a BandwidthAction
func GenerateOrderLimit(ctx context.Context, action pb.BandwidthAction, satID *identity.FullIdentity, upID *identity.FullIdentity, expiration time.Duration) (_ *pb.OrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)
	serialNum, err := uuid.New()
	if err != nil {
		return nil, err
	}
	pba := &pb.OrderLimit{
		SatelliteId:       satID.ID,
		UplinkId:          upID.ID,
		ExpirationUnixSec: time.Now().Add(expiration).Unix(),
		SerialNumber:      serialNum.String(),
		Action:            action,
		CreatedUnixSec:    time.Now().Unix(),
	}

	return pba, auth.SignMessage(ctx, pba, *satID)
}

//GenerateOrder creates a signed Order from a OrderLimit
func GenerateOrder(ctx context.Context, pba *pb.OrderLimit, storageNodeID storj.NodeID, upID *identity.FullIdentity, total int64) (_ *pb.Order, err error) {
	defer mon.Task()(&ctx)(&err)
	rba := &pb.Order{
		PayerAllocation: *pba,
		StorageNodeId:   storageNodeID,
		Total:           total,
	}
	// Combine Signature and Data for Order
	return rba, auth.SignMessage(ctx, rba, *upID)
}
