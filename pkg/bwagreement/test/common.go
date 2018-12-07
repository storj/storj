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
	"github.com/zeebo/errs"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/pb"
)

//GeneratePayerBandwidthAllocation creates a signed PayerBandwidthAllocation from a PayerBandwidthAllocation_Action
func GeneratePayerBandwidthAllocation(action pb.PayerBandwidthAllocation_Action, satelliteKey crypto.PrivateKey) (*pb.PayerBandwidthAllocation, error) {
	satelliteKeyEcdsa, ok := satelliteKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errs.New("Satellite Private Key is not a valid *ecdsa.PrivateKey")
	}

	// Generate PayerBandwidthAllocation_Data
	data, _ := proto.Marshal(
		&pb.PayerBandwidthAllocation_Data{
			SatelliteId:       teststorj.NodeIDFromString("SatelliteID"),
			UplinkId:          teststorj.NodeIDFromString("UplinkID"),
			ExpirationUnixSec: time.Now().Add(time.Hour * 24 * 10).Unix(),
			SerialNumber:      "SerialNumber",
			Action:            action,
			CreatedUnixSec:    time.Now().Unix(),
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
func GenerateRenterBandwidthAllocation(pba *pb.PayerBandwidthAllocation, uplinkKey crypto.PrivateKey) (*pb.RenterBandwidthAllocation, error) {
	// get "Uplink" Public Key
	uplinkKeyEcdsa, ok := uplinkKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errs.New("Uplink Private Key is not a valid *ecdsa.PrivateKey")
	}

	pubbytes, err := x509.MarshalPKIXPublicKey(&uplinkKeyEcdsa.PublicKey)
	if err != nil {
		return nil, errs.New("Could not generate byte array from Uplink Public key: %+v", err)
	}

	// Generate RenterBandwidthAllocation_Data
	data, _ := proto.Marshal(
		&pb.RenterBandwidthAllocation_Data{
			PayerAllocation: pba,
			PubKey:          pubbytes, // TODO: Take this out. It will be kept in a database on the satellite
			StorageNodeId:   teststorj.NodeIDFromString("StorageNodeID"),
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
