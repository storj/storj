// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testbwagreement

import (
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

//GeneratePayerBandwidthAllocation creates a signed PayerBandwidthAllocation from a BandwidthAction
func GeneratePayerBandwidthAllocation(action pb.BandwidthAction, satID *identity.FullIdentity, upID *identity.FullIdentity, expiration time.Duration) (*pb.PayerBandwidthAllocation, error) {
	serialNum, err := uuid.New()
	if err != nil {
		return nil, err
	}
	pba := &pb.PayerBandwidthAllocation{
		SatelliteId:       satID.ID,
		UplinkId:          upID.ID,
		ExpirationUnixSec: time.Now().Add(expiration).Unix(),
		SerialNumber:      serialNum.String(),
		Action:            action,
		CreatedUnixSec:    time.Now().Unix(),
	}

	return pba, auth.SignMessage(pba, *satID)
}

//GenerateRenterBandwidthAllocation creates a signed RenterBandwidthAllocation from a PayerBandwidthAllocation
func GenerateRenterBandwidthAllocation(pba *pb.PayerBandwidthAllocation, storageNodeID storj.NodeID, upID *identity.FullIdentity, total int64) (*pb.RenterBandwidthAllocation, error) {
	rba := &pb.RenterBandwidthAllocation{
		PayerAllocation: *pba,
		StorageNodeId:   storageNodeID,
		Total:           total,
	}
	// Combine Signature and Data for RenterBandwidthAllocation
	return rba, auth.SignMessage(rba, *upID)
}
