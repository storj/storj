// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testbwagreement

import (
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

//GenerateOrderLimit creates a signed OrderLimit from a BandwidthAction
func GenerateOrderLimit(action pb.BandwidthAction, satID *identity.FullIdentity, upID *identity.FullIdentity, expiration time.Duration) (*pb.OrderLimit, error) {
	serialNum, err := uuid.New()
	if err != nil {
		return nil, err
	}
	pba := &pb.OrderLimit{PBA: &pb.PBA{
		SatelliteId:       satID.ID,
		UplinkId:          upID.ID,
		ExpirationUnixSec: time.Now().Add(expiration).Unix(),
		SerialNumber:      serialNum.String(),
		Action:            action,
		CreatedUnixSec:    time.Now().Unix(),
	}}

	return pba, pba.Sign(satID.Key)
}

//GenerateOrder creates a signed Order from a OrderLimit
func GenerateOrder(pba *pb.OrderLimit, storageNodeID storj.NodeID, upID *identity.FullIdentity, total int64) (*pb.Order, error) {
	rba := &pb.Order{RBA: &pb.RBA{
		OrderLimit:    *pba,
		StorageNodeId: storageNodeID,
		Total:         total,
	}}
	// Combine Signature and Data for Order
	return rba, rba.Sign(upID.Key)
}
