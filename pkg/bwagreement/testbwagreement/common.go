// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testbwagreement

import (
	"context"
	"crypto"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/uplinkdb"
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

	err = auth.SignMessage(pba, *satID)
	if err != nil {
		return nil, err
	}

	// retrieve uplink's pub key
	// pubbytes, err := getUplinkPubKey(upID.Key)
	// if err != nil {
	// 	return nil, errs.New("Uplink Private Key is not a valid *ecdsa.PrivateKey")
	// }

	// store the corresponding uplink's id and public key into uplinkDB db
	// ctx := context.Background()
	// _, err = upldb.GetPublicKey(ctx, upID.ID)
	// if err != nil {
	// 	// no previous entry exists
	// 	err = upldb.SavePublicKey(context.Background(), uplinkdb.Agreement{ID: upID.ID, PublicKey: upID.Leaf.PublicKey})
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	return pba, nil
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

//SavePayerBandwidthAllocation creates a new entry of nodeid and corresponding publickey
func SavePayerBandwidthAllocation(upldb uplinkdb.DB, pba *pb.PayerBandwidthAllocation, publicKey crypto.PublicKey) error {
	// store the corresponding uplink's id and public key into uplinkDB db
	ctx := context.Background()
	_, err := upldb.GetPublicKey(ctx, pba.UplinkId)
	if err != nil {
		// no previous entry exists
		err = upldb.SavePublicKey(ctx, pba.UplinkId, publicKey)
		if err != nil {
			return err
		}
	}

	return nil
}

// // get uplink's public key
// func getUplinkPubKey(uplinkKey crypto.PrivateKey) ([]byte, error) {

// 	// get "Uplink" Public Key
// 	uplinkKeyEcdsa, ok := uplinkKey.(*ecdsa.PrivateKey)
// 	if !ok {
// 		return nil, errs.New("Uplink Private Key is not a valid *ecdsa.PrivateKey")
// 	}

// 	pubbytes, err := x509.MarshalPKIXPublicKey(&uplinkKeyEcdsa.PublicKey)
// 	if err != nil {
// 		return nil, errs.New("Could not generate byte array from Uplink Public key: %+v", err)
// 	}

// 	return pubbytes, nil
// }
