// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gtank/cryptopasta"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

//GeneratePayerBandwidthAllocation creates a signed PayerBandwidthAllocation from a PayerBandwidthAllocation_Action
func GeneratePayerBandwidthAllocation(action pb.PayerBandwidthAllocation_Action, satelliteKey crypto.PrivateKey, uplinkKey crypto.PrivateKey, expiration time.Duration) (*pb.PayerBandwidthAllocation, error) {
	satelliteKeyEcdsa, ok := satelliteKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errs.New("Satellite Private Key is not a valid *ecdsa.PrivateKey")
	}

	pubbytes, err := getUplinkPubKey(uplinkKey)
	if err != nil {
		return nil, errs.New("Uplink Private Key is not a valid *ecdsa.PrivateKey")
	}

	serialNum, err := uuid.New()
	if err != nil {
		return nil, err
	}

	// Generate PayerBandwidthAllocation_Data
	data, _ := proto.Marshal(
		&pb.PayerBandwidthAllocation_Data{
			SatelliteId:       teststorj.NodeIDFromString("SatelliteID"),
			UplinkId:          teststorj.NodeIDFromString("UplinkID"),
			ExpirationUnixSec: time.Now().Add(expiration).Unix(),
			SerialNumber:      serialNum.String(),
			Action:            action,
			CreatedUnixSec:    time.Now().Unix(),
			PubKey:            pubbytes,
		},
	)

	// Sign the PayerBandwidthAllocation_Data with the "Satellite" Private Key
	s, err := cryptopasta.Sign(data, satelliteKeyEcdsa)
	if err != nil {
		return nil, errs.New("Failed to sign PayerBandwidthAllocation_Data with satellite Private Key: %+v", err)
	}

	// Combine Signature and Data for PayerBandwidthAllocation
	return &pb.PayerBandwidthAllocation{
		Data:      data,
		Signature: s,
	}, nil
}

//GenerateRenterBandwidthAllocation creates a signed RenterBandwidthAllocation from a PayerBandwidthAllocation
func GenerateRenterBandwidthAllocation(pba *pb.PayerBandwidthAllocation, storageNodeID storj.NodeID, uplinkKey crypto.PrivateKey) (*pb.RenterBandwidthAllocation, error) {
	// get "Uplink" Public Key
	uplinkKeyEcdsa, ok := uplinkKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errs.New("Uplink Private Key is not a valid *ecdsa.PrivateKey")
	}

	// Generate RenterBandwidthAllocation_Data
	data, _ := proto.Marshal(
		&pb.RenterBandwidthAllocation_Data{
			PayerAllocation: pba,
			StorageNodeId:   storageNodeID,
			Total:           int64(666),
		},
	)

	// Sign the PayerBandwidthAllocation_Data with the "Uplink" Private Key
	s, err := cryptopasta.Sign(data, uplinkKeyEcdsa)
	if err != nil {
		return nil, errs.New("Failed to sign RenterBandwidthAllocation_Data with uplink Private Key: %+v", err)
	}

	// Combine Signature and Data for RenterBandwidthAllocation
	return &pb.RenterBandwidthAllocation{
		Signature: s,
		Data:      data,
	}, nil
}

// get uplink's public key
func getUplinkPubKey(uplinkKey crypto.PrivateKey) ([]byte, error) {

	// get "Uplink" Public Key
	uplinkKeyEcdsa, ok := uplinkKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errs.New("Uplink Private Key is not a valid *ecdsa.PrivateKey")
	}

	pubbytes, err := x509.MarshalPKIXPublicKey(&uplinkKeyEcdsa.PublicKey)
	if err != nil {
		return nil, errs.New("Could not generate byte array from Uplink Public key: %+v", err)
	}

	return pubbytes, nil
}
