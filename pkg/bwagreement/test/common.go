// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"crypto/ecdsa"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gtank/cryptopasta"
	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

//GeneratePayerBandwidthAllocation creates a signed PayerBandwidthAllocation from a PayerBandwidthAllocation_Action
func GeneratePayerBandwidthAllocation(action pb.PayerBandwidthAllocation_Action, satID *identity.FullIdentity, upID *identity.FullIdentity, expiration time.Duration) (*pb.PayerBandwidthAllocation, error) {
	serialNum, err := uuid.New()
	if err != nil {
		return nil, err
	}
	// Generate PayerBandwidthAllocation_Data
	data, _ := proto.Marshal(
		&pb.PayerBandwidthAllocation_Data{
			SatelliteId:       satID.ID,
			UplinkId:          upID.ID,
			ExpirationUnixSec: time.Now().Add(expiration).Unix(),
			SerialNumber:      serialNum.String(),
			Action:            action,
			CreatedUnixSec:    time.Now().Unix(),
		},
	)
	// Sign the PayerBandwidthAllocation_Data with the "Satellite" Private Key
	satPrivECDSA, ok := satID.Key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, pb.SatPrivKey.Wrap(pb.ECDSA)
	}
	s, err := cryptopasta.Sign(data, satPrivECDSA)
	if err != nil {
		return nil, pb.UpPrivKey.Wrap(pb.Sign.Wrap(err))
	}
	certs := [][]byte{satID.Leaf.Raw, satID.CA.Raw}
	certs = append(certs, satID.RestChainRaw()...) //todo: do we need RestChain?
	// Combine Signature and Data for PayerBandwidthAllocation
	return &pb.PayerBandwidthAllocation{
		Data:      data,
		Signature: s,
		Certs:     certs,
	}, nil
}

//GenerateRenterBandwidthAllocation creates a signed RenterBandwidthAllocation from a PayerBandwidthAllocation
func GenerateRenterBandwidthAllocation(pba *pb.PayerBandwidthAllocation, storageNodeID storj.NodeID, upID *identity.FullIdentity, total int64) (*pb.RenterBandwidthAllocation, error) {
	// Generate RenterBandwidthAllocation_Data
	data, _ := proto.Marshal(
		&pb.RenterBandwidthAllocation_Data{
			PayerAllocation: pba,
			StorageNodeId:   storageNodeID,
			Total:           total,
		},
	)
	// Sign the PayerBandwidthAllocation_Data with the "Uplink" Private Key
	upPrivECDSA, ok := upID.Key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, pb.UpPrivKey.Wrap(pb.ECDSA)
	}
	s, err := cryptopasta.Sign(data, upPrivECDSA)
	if err != nil {
		return nil, pb.UpPrivKey.Wrap(pb.Sign.Wrap(err))
	}
	certs := [][]byte{upID.Leaf.Raw, upID.CA.Raw}
	certs = append(certs, upID.RestChainRaw()...) //todo: do we need RestChain?
	// Combine Signature and Data for RenterBandwidthAllocation
	return &pb.RenterBandwidthAllocation{
		Signature: s,
		Data:      data,
		Certs:     certs,
	}, nil
}
